package xelon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

const providerIDPrefix = ProviderName + "://"

var _ cloudprovider.InstancesV2 = (*instances)(nil)

type xelonNode struct {
	localVMID string
	name      string
	nodeType  string
}

type instances struct {
	client    *clients
	clusterID string

	nodes      []xelonNode
	lastUpdate time.Time
	ttl        time.Duration

	sync.RWMutex
}

func newInstances(clients *clients, clusterID string) cloudprovider.InstancesV2 {
	return &instances{
		client:    clients,
		clusterID: clusterID,

		nodes: make([]xelonNode, 0),
		ttl:   15 * time.Second,
	}
}

func (i *instances) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	_, err := i.lookupXelonNode(ctx, node)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (i *instances) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	_, err := i.lookupXelonNode(ctx, node)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			return false, nil
		}
		return false, err
	}
	return false, nil
}

func (i *instances) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	meta := &cloudprovider.InstanceMetadata{}
	if node == nil {
		return meta, nil
	}

	xn, err := i.lookupXelonNode(ctx, node)
	if err != nil {
		return meta, err
	}

	meta.ProviderID = fmt.Sprintf("%s%s", providerIDPrefix, xn.localVMID)
	meta.NodeAddresses = node.Status.Addresses
	meta.InstanceType = xn.nodeType

	klog.V(5).InfoS("Setting instance metadata for node", "node", node.Name, "metadata", meta)

	return meta, nil
}

func (i *instances) lookupXelonNode(ctx context.Context, node *v1.Node) (*xelonNode, error) {
	err := i.refreshNodes(ctx)
	if err != nil {
		return nil, err
	}

	providerID := node.Spec.ProviderID
	if providerID != "" && isXelonProviderID(providerID) {
		klog.V(5).InfoS("Use providerID to get Xelon node", "provider_id", providerID)

		localVMID, err := parseProviderID(providerID)
		if err != nil {
			return nil, err
		}
		xn, err := i.getXelonNodeByLocalVMID(localVMID)
		if err != nil {
			return nil, err
		}
		return xn, nil
	} else {
		klog.V(5).InfoS("Use name to get Xelon node", "name", node.Name)

		xn, err := i.getXelonNodeByName(node.Name)
		if err != nil {
			return nil, err
		}
		return xn, nil
	}
}

// refreshNodes conditionally loads all control plane nodes, cluster pool nodes from Xelon API
// and caches them. It does not refresh if the last update happened less than 'nodeCache.ttl' ago.
func (i *instances) refreshNodes(ctx context.Context) error {
	i.Lock()
	defer i.Unlock()

	sinceLastUpdate := time.Since(i.lastUpdate)
	if sinceLastUpdate < i.ttl {
		klog.V(2).InfoS("Skip refreshing nodes", "since_last_update", sinceLastUpdate, "ttl", i.ttl)
		return nil
	}

	klog.V(5).InfoS("Getting control planes from Xelon API", "cluster_id", i.clusterID)
	controlPlane, _, err := i.client.xelon.Kubernetes.ListControlPlanes(ctx, i.clusterID)
	if err != nil {
		return err
	}
	klog.V(5).InfoS("Got control planes from Xelon API", "data", controlPlane)
	var controlPlaneNodes []xelonNode
	for _, controlPlaneNode := range controlPlane.Nodes {
		controlPlaneNodes = append(controlPlaneNodes, xelonNode{
			localVMID: controlPlaneNode.LocalVMID,
			name:      controlPlaneNode.Name,
			nodeType:  getNodeTypeFromControlPlaneNode(controlPlane),
		})
	}

	klog.V(5).InfoS("Getting cluster pools from Xelon API", "cluster_id", i.clusterID)
	clusterPools, _, err := i.client.xelon.Kubernetes.ListClusterPools(ctx, i.clusterID)
	if err != nil {
		return err
	}
	klog.V(5).InfoS("Got cluster pools from Xelon API", "data", clusterPools)
	var clusterPoolNodes []xelonNode
	for _, clusterPool := range clusterPools {
		for _, clusterPoolNode := range clusterPool.Nodes {
			clusterPoolNodes = append(clusterPoolNodes, xelonNode{
				localVMID: clusterPoolNode.LocalVMID,
				name:      clusterPoolNode.Name,
				nodeType:  getNodeTypeFromClusterPool(&clusterPool),
			})
		}
	}

	i.nodes = slices.Concat(controlPlaneNodes, clusterPoolNodes)
	i.lastUpdate = time.Now()

	return nil
}

func (i *instances) getXelonNodeByLocalVMID(localVMID string) (*xelonNode, error) {
	for _, node := range i.nodes {
		if node.localVMID == localVMID {
			return &node, nil
		}
	}

	return nil, cloudprovider.InstanceNotFound
}

func (i *instances) getXelonNodeByName(name string) (*xelonNode, error) {
	for _, node := range i.nodes {
		if node.name == name {
			return &node, nil
		}
	}

	return nil, cloudprovider.InstanceNotFound
}

// getNodeTypeFromControlPlaneNode formats a node type from control plane parameters
// in the following form <cpu_info>-<memory_info>-<disk_info>:
//   - cpu_info: shows CPU core count (e.g. c2c - 2 cores)
//   - memory_info: shows RAM in gigabytes (e.g. m4g - 4 GB)
//   - disk_info: shows disk size in gigabytes (e.g. d50g - 50 GB)
func getNodeTypeFromControlPlaneNode(controlPlane *xelon.ClusterControlPlane) string {
	if controlPlane == nil {
		return ""
	}
	return fmt.Sprintf("c%dc-m%dg-d%dg", controlPlane.CPUCoreCount, controlPlane.Memory, controlPlane.DiskSize)
}

// getNodeTypeFromClusterPool formats a node type from cluster pool parameters
// in the following form <cpu_info>-<memory_info>-<disk_info>:
//   - cpu_info: shows CPU core count (e.g. c2c - 2 cores)
//   - memory_info: shows RAM in gigabytes (e.g. m4g - 4 GB)
//   - disk_info: shows disk size in gigabytes (e.g. d50g - 50 GB)
func getNodeTypeFromClusterPool(clusterPool *xelon.ClusterPool) string {
	if clusterPool == nil {
		return ""
	}
	return fmt.Sprintf("c%dc-m%dg-d%dg", clusterPool.CPUCoreCount, clusterPool.Memory, clusterPool.DiskSize)
}

func parseProviderID(providerID string) (string, error) {
	if !isXelonProviderID(providerID) {
		return "", fmt.Errorf("invalid provider ID: %s", providerID)
	}
	return strings.TrimPrefix(providerID, providerIDPrefix), nil
}

func isXelonProviderID(providerID string) bool {
	return strings.HasPrefix(providerID, providerIDPrefix)
}
