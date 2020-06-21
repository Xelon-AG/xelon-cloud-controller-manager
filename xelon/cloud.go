package xelon

import (
	"fmt"
	"io"
	"os"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

const (
	ProviderName string = "xelon"
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
	token := os.Getenv("XELON_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("%s must be set in the environment (use k8s secret)", token)
	}

	xelonClient := xelon.NewClient(token)
	// TODO: set correct user agent
	if apiUrl := os.Getenv("XELON_API_URL"); apiUrl != "" {
		xelonClient.SetBaseURL(apiUrl)
	}

	return &cloud{
		client:        xelonClient,
		loadbalancers: newLoadBalancers(xelonClient),
	}, nil
}

func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadbalancers, false
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
