package xelon

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

const (
	ProviderName string = "xelon"

	xelonAPIURLEnv    string = "XELON_API_URL"
	xelonClusterIDEnv string = "XELON_CLUSTER_ID"
	xelonTokenEnv     string = "XELON_TOKEN"
)

type cloud struct {
	client        *xelon.Client
	loadbalancers cloudprovider.LoadBalancer
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud()
	})
}

func newCloud() (cloudprovider.Interface, error) {
	token := os.Getenv(xelonTokenEnv)
	if token == "" {
		return nil, fmt.Errorf("environment variable %q is required (use k8s secret)", xelonTokenEnv)
	}

	clusterID := os.Getenv(xelonClusterIDEnv)
	if clusterID == "" {
		return nil, fmt.Errorf("environment variable %q is required", xelonClusterIDEnv)
	}

	xelonClient := xelon.NewClient(token)
	xelonClient.SetUserAgent("xelon-cloud-controller-manager")

	if apiURL := os.Getenv(xelonAPIURLEnv); apiURL != "" {
		xelonClient.SetBaseURL(apiURL)
	}

	tenant, _, err := xelonClient.Tenant.Get(context.Background())
	if err != nil {
		return nil, err
	}

	return &cloud{
		client:        xelonClient,
		loadbalancers: newLoadBalancers(xelonClient, tenant.TenantID, clusterID),
	}, nil
}

func (c *cloud) Initialize(_ cloudprovider.ControllerClientBuilder, _ <-chan struct{}) {
}

func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadbalancers, true
}

func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (c *cloud) ProviderName() string {
	klog.V(5).Info("called ProviderName")
	return ProviderName
}

func (c *cloud) HasClusterID() bool {
	return false
}
