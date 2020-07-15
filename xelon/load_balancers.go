package xelon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

const (
	annotationXelonLoadBalancerID = "kubernetes.xelon.ch/load-balancer-id"

	annotationXelonLoadBalancerName = "service.beta.kubernetes.io/xelon-loadbalancer-name"

	annotationXelonHealthCheckPath                   = "service.beta.kubernetes.io/xelon-loadbalancer-healthcheck-path"
	annotationXelonHealthCheckPort                   = "service.beta.kubernetes.io/xelon-loadbalancer-healthcheck-port"
	annotationXelonHealthCheckIntervalSeconds        = "service.beta.kubernetes.io/xelon-loadbalancer-healthcheck-interval-seconds"
	annotationXelonHealthCheckResponseTimeoutSeconds = "service.beta.kubernetes.io/xelon-loadbalancer-healthcheck-response-timeout-seconds"
	annotationXelonHealthCheckUnhealthyThreshold     = "service.beta.kubernetes.io/xelon-loadbalancer-healthcheck-unhealthy-threshold"
	annotationXelonHealthCheckHealthyThreshold       = "service.beta.kubernetes.io/xelon-loadbalancer-healthcheck-healthy-threshold"
)

var errLoadBalancerNotFound = errors.New("loadbalancer not found")

type loadBalancers struct {
	client    *xelon.Client
	tenantID  string
	clusterID string
}

func newLoadBalancers(client *xelon.Client, tenantID, clusterID string) cloudprovider.LoadBalancer {
	return &loadBalancers{
		client:    client,
		tenantID:  tenantID,
		clusterID: clusterID,
	}
}

func (l *loadBalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lb, err := l.retrieveAndAnnotateLoadBalancer(ctx, service)
	if err != nil {
		if err == errLoadBalancerNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.IP,
			},
		},
	}, true, nil
}

func (l *loadBalancers) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	return getLoadBalancerName(service)
}

func (l *loadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	lbRequest, err := l.buildCreateLoadBalancerRequest(ctx, service, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to build load balancer request: %s", err)
	}

	var lb *xelon.LoadBalancer
	lb, err = l.retrieveAndAnnotateLoadBalancer(ctx, service)
	switch err {
	case nil:
		// LB existing
		_, err := l.updateLoadBalancer(ctx, lb, service, nodes)
		if err != nil {
			return nil, err
		}

	case errLoadBalancerNotFound:
		// LB missing
		_, _, err := l.client.LoadBalancer.Create(ctx, l.tenantID, lbRequest)
		logLBInfo("CREATE", lbRequest, 2)
		if err != nil {
			return nil, err
		}
		lbs, _, err := l.client.LoadBalancer.List(ctx, l.tenantID)
		if err != nil {
			return nil, err
		}
		for _, v := range lbs {
			if v.Name == lbRequest.Name {
				lb = &v
			}
		}

		if lb != nil {
			updateServiceAnnotation(service, annotationXelonLoadBalancerID, lb.LocalID)
		}

	default:
		// unrecoverable LB retrieval error
		return nil, err
	}

	if lb == nil {
		return nil, fmt.Errorf("load-balancer is still not found")
	}

	if lb.Health != "green" {
		return nil, fmt.Errorf("load-balancer is not yet active (current status: %s)", lb.Health)
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.IP,
			},
		},
	}, nil
}

func (l *loadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	lb, err := l.retrieveAndAnnotateLoadBalancer(ctx, service)
	if err != nil {
		return err
	}
	_, err = l.updateLoadBalancer(ctx, lb, service, nodes)
	return err
}

func (l *loadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	lb, err := l.retrieveLoadBalancer(ctx, service)
	if err != nil {
		if err == errLoadBalancerNotFound {
			return nil
		}
		return err
	}

	resp, err := l.client.LoadBalancer.Delete(ctx, l.tenantID, lb.LocalID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete load balancer: %s", err)
	}

	return nil
}

func (l *loadBalancers) retrieveAndAnnotateLoadBalancer(ctx context.Context, service *v1.Service) (*xelon.LoadBalancer, error) {
	lb, err := l.retrieveLoadBalancer(ctx, service)
	if err != nil {
		// Return bare error to easily compare for errLBNotFound. Converting to
		// a full error type doesn't seem worth it.
		return nil, err
	}

	updateServiceAnnotation(service, annotationXelonLoadBalancerID, lb.LocalID)

	return lb, nil
}

func (l *loadBalancers) retrieveLoadBalancer(ctx context.Context, service *v1.Service) (*xelon.LoadBalancer, error) {
	// id := getLoadBalancerID(service)
	// if len(id) > 0 {
	// 	klog.V(2).Infof("looking up load-balancer for service %s/%s by ID %s", service.Namespace, service.Name, id)
	//
	// 	return l.findLoadBalancerByID(ctx, id)
	// }

	allLBs, err := l.getLoadBalancers(ctx)
	if err != nil {
		return nil, err
	}

	lb := findLoadBalancerByName(service, allLBs)
	if lb == nil {
		return nil, errLoadBalancerNotFound
	}

	return lb, nil
}

// func (l *loadBalancers) findLoadBalancerByID(ctx context.Context, id string) (*xelon.LoadBalancer, error) {
// 	lb, resp, err := l.client.LoadBalancer.Get(ctx, l.tenantID, id)
// 	if err != nil {
// 		if resp != nil && resp.StatusCode == http.StatusNotFound {
// 			return nil, errLoadBalancerNotFound
// 		}
//
// 		return nil, fmt.Errorf("failed to get load-balancer by ID %s: %s", id, err)
// 	}
// 	return lb, nil
// }

func (l *loadBalancers) getLoadBalancers(ctx context.Context) ([]xelon.LoadBalancer, error) {
	lbs, _, err := l.client.LoadBalancer.List(ctx, l.tenantID)
	if err != nil {
		return nil, err
	}
	return lbs, nil
}

func findLoadBalancerByName(service *v1.Service, allLBs []xelon.LoadBalancer) *xelon.LoadBalancer {
	customName := getLoadBalancerName(service)
	legacyName := getLoadBalancerLegacyName(service)
	candidates := []string{customName}
	if legacyName != customName {
		candidates = append(candidates, legacyName)
	}

	klog.V(2).Infof("Looking up load-balancer for service %s/%s by name (candidates: %s)", service.Namespace, service.Name, strings.Join(candidates, ", "))

	for _, lb := range allLBs {
		for _, candidate := range candidates {
			if lb.Name == candidate {
				return &lb
			}
		}
	}

	return nil
}

func getLoadBalancerName(service *v1.Service) string {
	name := service.Annotations[annotationXelonLoadBalancerName]

	if len(name) > 0 {
		return name
	}

	return getLoadBalancerLegacyName(service)
}

func getLoadBalancerLegacyName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

func getLoadBalancerID(service *v1.Service) string {
	return service.ObjectMeta.Annotations[annotationXelonLoadBalancerID]
}

func updateServiceAnnotation(service *v1.Service, annotationName, annotationValue string) {
	if service.ObjectMeta.Annotations == nil {
		service.ObjectMeta.Annotations = map[string]string{}
	}
	service.ObjectMeta.Annotations[annotationName] = annotationValue
}

// buildCreateLoadBalancerRequest returns a *xelon.LoadBalancerCreateRequest to balance requests for service across nodes.
func (l *loadBalancers) buildCreateLoadBalancerRequest(ctx context.Context, service *v1.Service, nodes []*v1.Node) (*xelon.LoadBalancerCreateRequest, error) {
	lbName := getLoadBalancerName(service)

	forwardingRules, err := buildForwardingRules(service)
	if err != nil {
		return nil, err
	}

	return &xelon.LoadBalancerCreateRequest{
		ForwardingRules: forwardingRules,
		Name:            lbName,
		Type:            1,
		ServerID:        []string{l.clusterID},
	}, nil
}

// buildUpdateLoadBalancerRequest returns a *xelon.LoadBalancerUpdateForwardingRulesRequest to balance requests for service across nodes.
func (l *loadBalancers) buildUpdateLoadBalancerRequest(ctx context.Context, service *v1.Service) (*xelon.LoadBalancerUpdateForwardingRulesRequest, error) {
	forwardingRules, err := buildForwardingRules(service)
	if err != nil {
		return nil, err
	}

	return &xelon.LoadBalancerUpdateForwardingRulesRequest{
		ForwardingRules: forwardingRules,
	}, nil
}

// buildForwardingRules returns the forwarding rules of the Load Balancer of service.
func buildForwardingRules(service *v1.Service) ([]xelon.LoadBalancerForwardingRule, error) {
	var forwardingRules []xelon.LoadBalancerForwardingRule

	for _, port := range service.Spec.Ports {
		forwardingRule, err := buildForwardingRule(&port)
		if err != nil {
			return nil, err
		}
		forwardingRules = append(forwardingRules, *forwardingRule)
	}

	return forwardingRules, nil
}

func buildForwardingRule(port *v1.ServicePort) (*xelon.LoadBalancerForwardingRule, error) {
	var forwardingRule xelon.LoadBalancerForwardingRule

	forwardingRule.Ports = []int{int(port.Port), int(port.NodePort)}

	return &forwardingRule, nil
}

func (l *loadBalancers) updateLoadBalancer(ctx context.Context, lb *xelon.LoadBalancer, service *v1.Service, nodes []*v1.Node) (*xelon.LoadBalancer, error) {
	// call buildCreateLoadBalancerRequest for its error checking; we have to call it
	// again just before actually updating the loadbalancer in case
	// checkAndUpdateLBAndServiceCerts modifies the service
	_, err := l.buildCreateLoadBalancerRequest(ctx, service, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to build load-balancer request: %s", err)
	}

	lbRequest, err := l.buildUpdateLoadBalancerRequest(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("failed to build load-balancer request (post-certificate update): %s", err)
	}

	lbID := lb.LocalID
	_, _, err = l.client.LoadBalancer.UpdateForwardingRules(ctx, l.tenantID, lbID, lbRequest)
	if err != nil {
		logLBInfo("UPDATE", lbRequest, 2)
		return nil, fmt.Errorf("failed to update load-balancer with ID %s: %s", lbID, err)
	}
	logLBInfo("UPDATE", lbRequest, 2)

	return lb, nil
}

// logLBInfo wraps around klog and logs LB operation type and LB configuration info.
func logLBInfo(opType string, cfgInfo interface{}, logLevel klog.Level) {
	if cfgInfo != nil {
		klog.V(logLevel).Infof("Operation type: %v, Configuration info: %v", opType, cfgInfo)
	}
}
