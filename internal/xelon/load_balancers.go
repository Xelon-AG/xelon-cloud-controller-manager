package xelon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	apierrors "k8s.io/cloud-provider/api"
	"k8s.io/klog/v2"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

const (
	xelonLoadBalancerClusterStatusActive           = "Active"
	xelonLoadBalancerClusterStatusProvisioning     = "Provisioning"
	xelonLoadBalancerClusterVirtualIPStateReserved = "reserved"

	// serviceAnnotationLoadBalancerClusterID is the annotation used on the service
	// to identify Xelon load balancer cluster. Read-only.
	serviceAnnotationLoadBalancerClusterID = "kubernetes.xelon.ch/load-balancer-cluster-id"

	// serviceAnnotationLoadBalancerClusterName is the annotation used on the service
	// to identify Xelon load balancer cluster name. Read-only.
	// serviceAnnotationLoadBalancerClusterName = "kubernetes.xelon.ch/load-balancer-cluster-name"

	// serviceAnnotationLoadBalancerClusterVirtualIPID is the annotation used on the service
	// to identify Xelon load balancer cluster virtual IP. Read-only.
	serviceAnnotationLoadBalancerClusterVirtualIPID = "kubernetes.xelon.ch/load-balancer-cluster-virtual-ip-id"

	// serviceAnnotationLoadBalancerClusterForwardingRuleIDs is the annotation used on the service
	// to identify frontend forwarding rules for the virtual IP. Comma-separated, read-only.
	serviceAnnotationLoadBalancerClusterForwardingRuleIDs = "kubernetes.xelon.ch/load-balancer-cluster-forwarding-rule-ids"

	// serviceAnnotationLoadBalancerClusterCreatingEnabled is the annotation
	// used on the service to allow creation of new load balancer clusters.
	// serviceAnnotationLoadBalancerClusterCreatingEnabled = "service.beta.kubernetes.io/xelon-load-balancer-cluster-creating-enabled"

	// serviceAnnotationLoadBalancerClusterProxyProtocolVersion is the annotation
	// used on the service to allow to specify proxy protocol version.
	//
	//   - 0: default value, don't send proxy protocol to the backend
	//   - 1: Proxy Protocol version 1 (text format)
	//   - 2: Proxy Protocol version 2 (binary format)
	serviceAnnotationLoadBalancerClusterProxyProtocolVersion = "service.beta.kubernetes.io/xelon-load-balancer-cluster-proxy-protocol-version"
)

var (
	errLoadBalancerNotFound             = errors.New("load balancer not found")
	errLoadBalancerProvisioning         = errors.New("load balancer is being provisioned")
	errLoadBalancerNoVirtualIPAvailable = errors.New("load balancer cluster virtual ip is not available")

	_ cloudprovider.LoadBalancer = &loadBalancers{}
)

type loadBalancers struct {
	client *clients

	tenantID  string
	cloudID   string
	clusterID string

	*sync.RWMutex
}

// xelonLoadBalancer represents an abstraction to map cloudprovider.LoadBalancer
// and Xelon specific objects: load balancer cluster and virtual IP.
//   - cluster contains two (or more) virtual ip addresses
//   - valid cluster should be in "Active" status
//   - virtual ip address should have "free" state
//   - virtual ip addresses may be shared across different services (if services exposes different ports)
type xelonLoadBalancer struct {
	clusterID        string
	virtualIPID      string
	virtualIPAddress string
	forwardingRules  []xelon.LoadBalancerClusterForwardingRule
}

func newLoadBalancers(clients *clients, tenantID, cloudID, clusterID string) cloudprovider.LoadBalancer {
	return &loadBalancers{
		client:    clients,
		tenantID:  tenantID,
		cloudID:   cloudID,
		clusterID: clusterID,

		RWMutex: &sync.RWMutex{},
	}
}

func (l *loadBalancers) GetLoadBalancer(ctx context.Context, _ string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	logger := configureLogger(ctx, "GetLoadBalancer")

	xlb, err := l.retrieveXelonLoadBalancer(ctx, service)
	if err != nil {
		if errors.Is(err, errLoadBalancerNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	logger.WithValues("ip_address", xlb.virtualIPAddress).Info("Load balancer virtual IP address")

	return &v1.LoadBalancerStatus{
		Ingress: l.buildLoadBalancerStatusIngress(ctx, xlb, service),
	}, true, nil
}

func (l *loadBalancers) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

func (l *loadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	logger := klog.FromContext(ctx).WithValues("method", "EnsureLoadBalancer", "service", getServiceNameWithNamespace(service))

	xlb, err := l.retrieveXelonLoadBalancer(ctx, service)
	if err != nil {
		switch {
		case errors.Is(err, errLoadBalancerNotFound):
			logger.Info("create case does not supported yet")
			return nil, err

		case errors.Is(err, errLoadBalancerProvisioning):
			return nil, apierrors.NewRetryError("load balancer is currently being provisioned", 30*time.Second)

		default:
			// unrecoverable error
			return nil, err
		}
	}

	err = l.UpdateLoadBalancer(ctx, clusterName, service, nodes)
	if err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: l.buildLoadBalancerStatusIngress(ctx, xlb, service),
	}, nil
}

func (l *loadBalancers) UpdateLoadBalancer(ctx context.Context, _ string, service *v1.Service, _ []*v1.Node) error {
	xlb, err := l.retrieveXelonLoadBalancer(ctx, service)
	if err != nil {
		return err
	}

	err = l.updateLoadBalancer(ctx, xlb, service)
	if err != nil {
		return err
	}

	return nil
}

func (l *loadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, _ string, service *v1.Service) error {
	logger := configureLogger(ctx, "EnsureLoadBalancerDeleted")

	xlb, err := l.retrieveXelonLoadBalancer(ctx, service)
	if err != nil {
		return err
	}

	if xlb == nil {
		logger.Info("xelonLoadBalancer is empty, no rules delete needed")
		return nil
	}
	if xlb.forwardingRules == nil {
		logger.Info("no forwarding rules defined, no rules delete needed")
		return nil
	}

	var frontendRules []xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration
	for _, forwardingRule := range xlb.forwardingRules {
		if forwardingRule.Frontend != nil {
			frontendRules = append(frontendRules, *forwardingRule.Frontend)
		}
	}
	logger.WithValues("frontend_rules", frontendRules).Info("Following rules will be deleted")
	for _, frontendRule := range frontendRules {
		_, err := l.client.xelon.LoadBalancerClusters.DeleteForwardingRule(ctx, xlb.clusterID, xlb.virtualIPID, frontendRule.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *loadBalancers) retrieveXelonLoadBalancer(ctx context.Context, service *v1.Service) (xlb *xelonLoadBalancer, err error) {
	logger := configureLogger(ctx, "retrieveXelonLoadBalancer").WithValues(
		"service", getServiceNameWithNamespace(service),
	)
	patcher := newServicePatcher(l.client.k8s, service)
	defer func() { err = patcher.Patch(ctx) }()

	xlb = &xelonLoadBalancer{}

	// fetch all needed information about Xelon load balancer cluster
	if id, ok := service.Annotations[serviceAnnotationLoadBalancerClusterID]; ok && id != "" {
		logger.Info("Load balancer cluster id is specified", "id", id)

		loadBalancerCluster, err := l.fetchXelonLoadBalancerCluster(ctx, id)
		if err != nil {
			return nil, err
		}

		if loadBalancerCluster.Status == xelonLoadBalancerClusterStatusProvisioning {
			// special case for clusters in provisioning state, so EnsureLoadBalancer method can use retry error
			return nil, errLoadBalancerProvisioning
		}
		if loadBalancerCluster.Status != xelonLoadBalancerClusterStatusActive {
			return nil, fmt.Errorf("load balancer cluster is not active (current status: %v)", loadBalancerCluster.Status)
		}

		xlb.clusterID = loadBalancerCluster.ID
	} else {
		logger.Info("Load balancer cluster id is not specified, searching for a cluster that can be used for the service")

		loadBalancerCluster, err := l.findOrCreateXelonLoadBalancerCluster(ctx, service)
		if err != nil {
			return nil, err
		}

		if loadBalancerCluster.Status == xelonLoadBalancerClusterStatusProvisioning {
			// special case for clusters in provisioning state, so EnsureLoadBalancer method can use retry error
			return nil, errLoadBalancerProvisioning
		}
		if loadBalancerCluster.Status != xelonLoadBalancerClusterStatusActive {
			return nil, fmt.Errorf("load balancer cluster is not active (current status: %v)", loadBalancerCluster.Status)
		}

		xlb.clusterID = loadBalancerCluster.ID
		updateServiceAnnotation(service, serviceAnnotationLoadBalancerClusterID, loadBalancerCluster.ID)
	}

	// fetch all needed information about virtual IP from the load balancer cluster
	if id, ok := service.Annotations[serviceAnnotationLoadBalancerClusterVirtualIPID]; ok && id != "" {
		logger.Info("Load balancer cluster virtual ip is specified", "id", id)

		virtualIP, err := l.fetchXelonLoadBalancerVirtualIP(ctx, xlb.clusterID, id)
		if err != nil {
			return nil, err
		}

		xlb.virtualIPID = virtualIP.ID
		xlb.virtualIPAddress = virtualIP.IPAddress
	} else {
		logger.Info("Load balancer cluster virtual ip is not specified, searching for a virtual ip that can be used for the service")

		virtualIP, err := l.findXelonLoadBalancerClusterVirtualIP(ctx, xlb.clusterID, service)
		if err != nil {
			return nil, err
		}

		xlb.virtualIPID = virtualIP.ID
		xlb.virtualIPAddress = virtualIP.IPAddress

		updateServiceAnnotation(service, serviceAnnotationLoadBalancerClusterVirtualIPID, virtualIP.ID)
	}

	// fetch all needed information about forwarding rules
	if ids, ok := service.Annotations[serviceAnnotationLoadBalancerClusterForwardingRuleIDs]; ok && ids != "" {
		logger.Info("Forwarding rules are specified", "forwarding_rules_ids", ids)

		forwardingRules, err := l.fetchXelonLoadBalancerForwardingRules(ctx, xlb.clusterID, xlb.virtualIPID, ids)
		if err != nil {
			return nil, err
		}

		xlb.forwardingRules = forwardingRules
	}

	return xlb, nil
}

func (l *loadBalancers) fetchXelonLoadBalancerCluster(ctx context.Context, loadBalancerClusterID string) (*xelon.LoadBalancerCluster, error) {
	logger := configureLogger(ctx, "fetchXelonLoadBalancerCluster")

	loadBalancerCluster, resp, err := l.client.xelon.LoadBalancerClusters.Get(ctx, loadBalancerClusterID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			logger.Info("Load balancer cluster does not exist", "id", loadBalancerClusterID)
			return nil, errLoadBalancerNotFound
		}
		return nil, err
	}

	logger.Info("Load balancer cluster exists", "id", loadBalancerCluster.ID, "name", loadBalancerCluster.Name)

	return loadBalancerCluster, nil
}

func (l *loadBalancers) findOrCreateXelonLoadBalancerCluster(ctx context.Context, service *v1.Service) (*xelon.LoadBalancerCluster, error) {
	logger := configureLogger(ctx, "findOrCreateXelonLoadBalancerCluster").WithValues(
		"service", getServiceNameWithNamespace(service),
	)

	loadBalancerClusters, _, err := l.client.xelon.LoadBalancerClusters.List(ctx)
	if err != nil {
		return nil, err
	}

	var cluster *xelon.LoadBalancerCluster
	logger.Info("Searching for load balancer cluster", "kubernetes_cluster_id", l.clusterID)
	for _, loadBalancerCluster := range loadBalancerClusters {
		if loadBalancerCluster.KubernetesClusterID == l.clusterID {
			logger.Info("Found load balancer cluster", "id", loadBalancerCluster.ID, "name", loadBalancerCluster.Name)

			// skip non-active load balancer clusters
			if loadBalancerCluster.Status != xelonLoadBalancerClusterStatusActive {
				continue
			}

			_, err := l.findXelonLoadBalancerClusterVirtualIP(ctx, loadBalancerCluster.ID, service)
			if err != nil {
				if errors.Is(err, errLoadBalancerNoVirtualIPAvailable) {
					logger.Info("No virtual ip available")
					continue
				}
				return nil, err
			}
			cluster = &loadBalancerCluster
			break
		}
	}

	if cluster == nil {
		// create case is not supported yet
		logger.Info("Creating new load balancer cluster is not supported yet")
		return nil, errors.New("creating new load balancer cluster is not supported")
	} else {
		return cluster, nil
	}
}

func (l *loadBalancers) fetchXelonLoadBalancerVirtualIP(ctx context.Context, loadbalancerClusterID, virtualIPID string) (*xelon.LoadBalancerClusterVirtualIP, error) {
	logger := configureLogger(ctx, "fetchXelonLoadBalancerVirtualIP")

	virtualIP, resp, err := l.client.xelon.LoadBalancerClusters.GetVirtualIP(ctx, loadbalancerClusterID, virtualIPID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			logger.Info("Load balancer cluster virtual ip does not exist", "id", virtualIPID)
			return nil, errLoadBalancerNotFound
		}
		return nil, err
	}

	logger.Info("Load balancer cluster virtual ip exists", "id", virtualIP.ID, "address", virtualIP.IPAddress)

	return virtualIP, nil
}

func (l *loadBalancers) findXelonLoadBalancerClusterVirtualIP(ctx context.Context, loadBalancerClusterID string, service *v1.Service) (*xelon.LoadBalancerClusterVirtualIP, error) {
	logger := configureLogger(ctx, "findXelonLoadBalancerClusterVirtualIP").WithValues(
		"service", getServiceNameWithNamespace(service),
	)

	virtualIPs, _, err := l.client.xelon.LoadBalancerClusters.ListVirtualIPs(ctx, loadBalancerClusterID)
	if err != nil {
		return nil, err
	}
	for _, virtualIP := range virtualIPs {
		forwardingRules, _, err := l.client.xelon.LoadBalancerClusters.ListForwardingRules(ctx, loadBalancerClusterID, virtualIP.ID)
		if err != nil {
			return nil, err
		}

		if isVirtualIPAvailable(&virtualIP, forwardingRules, service) {
			logger.Info("Found available virtual IP", "id", virtualIP.ID, "address", virtualIP.IPAddress)
			return &virtualIP, nil
		}
	}

	return nil, errLoadBalancerNoVirtualIPAvailable
}

func (l *loadBalancers) fetchXelonLoadBalancerForwardingRules(ctx context.Context, loadbalancerClusterID, virtualIPID, forwardingRuleIDs string) ([]xelon.LoadBalancerClusterForwardingRule, error) {
	logger := configureLogger(ctx, "fetchXelonLoadBalancerForwardingRules")

	definedForwardingRuleIDs := strings.Split(forwardingRuleIDs, ",")

	forwardingRules, _, err := l.client.xelon.LoadBalancerClusters.ListForwardingRules(ctx, loadbalancerClusterID, virtualIPID)
	if err != nil {
		return nil, err
	}

	var ff []xelon.LoadBalancerClusterForwardingRule
	for _, forwardingRule := range forwardingRules {
		if forwardingRule.Frontend == nil {
			continue
		}
		if slices.Contains(definedForwardingRuleIDs, forwardingRule.Frontend.ID) {
			logger.WithValues(
				"rule_ids", definedForwardingRuleIDs, "id", forwardingRule.Frontend.ID,
			).Info("Found match for frontend forwarding rule")
			ff = append(ff, forwardingRule)
		}
	}

	return ff, nil
}

func (l *loadBalancers) updateLoadBalancer(ctx context.Context, xlb *xelonLoadBalancer, service *v1.Service) error {
	logger := configureLogger(ctx, "updateLoadBalancer").WithValues(
		"service", getServiceNameWithNamespace(service),
	)

	l.Lock()
	defer l.Unlock()

	patcher := newServicePatcher(l.client.k8s, service)
	defer func() { _ = patcher.Patch(ctx) }()

	// check proxy_protocol annotation
	protocolVersion := 0
	if protocolVersionAsString, ok := service.Annotations[serviceAnnotationLoadBalancerClusterProxyProtocolVersion]; ok && protocolVersionAsString != "" {
		parsedProtocolVersion, err := strconv.Atoi(protocolVersionAsString)
		if err != nil {
			return fmt.Errorf("could not convert proxy protocol version (%v) to integer", protocolVersionAsString)
		}
		protocolVersion = parsedProtocolVersion
		logger.Info("Proxy protocol annotation is defined and will be used for backend forwarding rules", "proxy_protocol", protocolVersion)
	}

	// get current state
	var currentForwardingRules []xelon.LoadBalancerClusterForwardingRule
	var currentForwardingRuleIDs []string
	if forwardingRuleIDs, ok := service.Annotations[serviceAnnotationLoadBalancerClusterForwardingRuleIDs]; ok && forwardingRuleIDs != "" {
		currentForwardingRuleIDs = strings.Split(forwardingRuleIDs, ",")
	}
	existingForwardingRules, _, err := l.client.xelon.LoadBalancerClusters.ListForwardingRules(ctx, xlb.clusterID, xlb.virtualIPID)
	if err != nil {
		return err
	}
	for _, existingForwardingRule := range existingForwardingRules {
		if slices.Contains(currentForwardingRuleIDs, existingForwardingRule.Frontend.ID) {
			currentForwardingRules = append(currentForwardingRules, existingForwardingRule)
		}
	}
	logger.Info("Fetched current state for forwarding rules", "current_forwarding_rules", currentForwardingRules)

	// get desired state
	var desiredForwardingRules []xelon.LoadBalancerClusterForwardingRule
	for _, port := range service.Spec.Ports {
		portNo := int(port.Port)
		forwardingRule := xelon.LoadBalancerClusterForwardingRule{
			Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: int(port.NodePort), ProxyProtocol: protocolVersion},
			Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: portNo},
		}
		desiredForwardingRules = append(desiredForwardingRules, forwardingRule)
	}
	logger.Info("Calculated desired state for forwarding rules", "desired_forwarding_rules", desiredForwardingRules)

	// calculate diff (reconcile)
	reconcileDiff := reconcile(currentForwardingRules, desiredForwardingRules)
	logger.Info("Calculate reconcile state",
		"rules_to_create", reconcileDiff.rulesToCreate,
		"rules_to_update", reconcileDiff.rulesToUpdate,
		"rules_to_delete", reconcileDiff.rulesToDelete,
	)

	var frontendRuleIDs []string
	if len(reconcileDiff.rulesToCreate) > 0 {
		logger.Info("Creating new forwarding rules", "payload", reconcileDiff.rulesToCreate)
		rules, _, err := l.client.xelon.LoadBalancerClusters.CreateForwardingRules(ctx, xlb.clusterID, xlb.virtualIPID, reconcileDiff.rulesToCreate)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if rule.Frontend == nil {
				continue
			}
			frontendRuleIDs = append(frontendRuleIDs, rule.Frontend.ID)
		}
	}

	if len(reconcileDiff.rulesToUpdate) > 0 {
		for _, ruleToUpdate := range reconcileDiff.rulesToUpdate {
			updateRequest := &xelon.LoadBalancerClusterForwardingRuleUpdateResponse{
				Port:          ruleToUpdate.Backend.Port,
				ProxyProtocol: ruleToUpdate.Backend.ProxyProtocol,
			}
			logger.Info("Updating existing forwarding backend rule", "payload", updateRequest)
			_, _, err := l.client.xelon.LoadBalancerClusters.UpdateForwardingRule(ctx, xlb.clusterID, xlb.virtualIPID, ruleToUpdate.Backend.ID, updateRequest)
			if err != nil {
				return err
			}
		}
	}

	if len(reconcileDiff.rulesToDelete) > 0 {
		logger.Info("Deleting forwarding rules", "payload", reconcileDiff.rulesToDelete)
		for _, ruleToDelete := range reconcileDiff.rulesToDelete {
			if ruleToDelete.Frontend == nil {
				continue
			}
			resp, err := l.client.xelon.LoadBalancerClusters.DeleteForwardingRule(ctx, xlb.clusterID, xlb.virtualIPID, ruleToDelete.Frontend.ID)
			if err != nil {
				if resp != nil && resp.StatusCode == http.StatusNotFound {
					logger.Info("Skipped removing not existing forwarding rule", "forwarding_rule_id", ruleToDelete.Frontend.ID)
				} else {
					return err
				}
			}
		}
	}

	// normalize ids
	forwardingRuleIDs := slices.Concat(currentForwardingRuleIDs, frontendRuleIDs)
	slices.Sort(forwardingRuleIDs)
	forwardingRuleIDs = slices.Compact(forwardingRuleIDs)
	if len(reconcileDiff.rulesToDelete) > 0 {
		for _, ruleToDelete := range reconcileDiff.rulesToDelete {
			if ruleToDelete.Frontend == nil {
				continue
			}
			forwardingRuleIDs = slices.DeleteFunc(forwardingRuleIDs, deleteByFrontendID(ruleToDelete.Frontend.ID))
		}
	}

	logger.Info("Applying forwarding rules annotation", "forwarding_rules_ids", strings.Join(forwardingRuleIDs, ","))
	updateServiceAnnotation(service, serviceAnnotationLoadBalancerClusterForwardingRuleIDs, strings.Join(forwardingRuleIDs, ","))

	return nil
}

func (l *loadBalancers) buildLoadBalancerStatusIngress(ctx context.Context, xlb *xelonLoadBalancer, service *v1.Service) []v1.LoadBalancerIngress {
	logger := configureLogger(ctx, "buildLoadBalancerStatusIngress").WithValues(
		"service", getServiceNameWithNamespace(service),
	)

	ipMode := v1.LoadBalancerIPModeVIP

	protocolVersion := 0
	if protocolVersionAsString, ok := service.Annotations[serviceAnnotationLoadBalancerClusterProxyProtocolVersion]; ok && protocolVersionAsString != "" {
		parsedProtocolVersion, err := strconv.Atoi(protocolVersionAsString)
		if err != nil {
			logger.Info("Could not convert proxy protocol version, fallback to 0")
		} else {
			protocolVersion = parsedProtocolVersion
			logger.Info("Proxy protocol annotation is defined and will be used for load balancer ingress", "proxy_protocol", protocolVersion)
		}
	}
	if protocolVersion > 0 {
		ipMode = v1.LoadBalancerIPModeProxy
	}

	return []v1.LoadBalancerIngress{{
		IP:     xlb.virtualIPAddress,
		IPMode: &ipMode,
	}}
}

func updateServiceAnnotation(service *v1.Service, annotationName, annotationValue string) {
	if service.ObjectMeta.Annotations == nil {
		service.ObjectMeta.Annotations = map[string]string{}
	}
	service.ObjectMeta.Annotations[annotationName] = annotationValue
}

func isVirtualIPAvailable(virtualIP *xelon.LoadBalancerClusterVirtualIP, forwardingRules []xelon.LoadBalancerClusterForwardingRule, service *v1.Service) bool {
	if service == nil {
		return false
	}
	if virtualIP == nil {
		return false
	}
	if virtualIP.State == xelonLoadBalancerClusterVirtualIPStateReserved {
		return false
	}

	// combine all frontend ports, so we can check it later
	var frontendPorts []int32
	for _, forwardingRule := range forwardingRules {
		if forwardingRule.Frontend != nil {
			frontendPorts = append(frontendPorts, int32(forwardingRule.Frontend.Port))
		}
	}

	// check if service's ports are already configured in forwarding rules
	for _, servicePort := range service.Spec.Ports {
		if slices.Contains(frontendPorts, servicePort.Port) {
			return false
		}
	}

	return true
}

func configureLogger(ctx context.Context, methodName string) logr.Logger {
	return klog.FromContext(ctx).V(2).WithValues("method", methodName)
}

func getServiceNameWithNamespace(service *v1.Service) string {
	return fmt.Sprintf("%v/%v", service.Namespace, service.Name)
}
