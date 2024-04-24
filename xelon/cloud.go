package xelon

import (
	"context"
	"fmt"
	"io"
	"os"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

const (
	ProviderName string = "xelon"

	xelonAPIURLEnv    string = "XELON_API_URL"
	xelonClientIDEnv  string = "XELON_CLIENT_ID"
	xelonCloudIDEnv   string = "XELON_CLOUD_ID"
	xelonClusterIDEnv string = "XELON_CLUSTER_ID"
	xelonTokenEnv     string = "XELON_TOKEN"
)

type cloud struct {
	client        *xelon.Client
	loadbalancers cloudprovider.LoadBalancer
}

func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	// TODO implement m
	panic("implement me")
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

	cloudID := os.Getenv(xelonCloudIDEnv)
	if cloudID == "" {
		return nil, fmt.Errorf("environment variable %q is required", xelonClusterIDEnv)
	}

	clusterID := os.Getenv(xelonClusterIDEnv)
	if clusterID == "" {
		return nil, fmt.Errorf("environment variable %q is required", xelonClusterIDEnv)
	}

	userAgent := "xelon-cloud-controller-manager"
	opts := []xelon.ClientOption{xelon.WithUserAgent(userAgent)}
	if apiURL := os.Getenv(xelonAPIURLEnv); apiURL != "" {
		opts = append(opts, xelon.WithBaseURL(apiURL))
	}
	if clientID := os.Getenv(xelonClientIDEnv); clientID != "" {
		opts = append(opts, xelon.WithClientID(clientID))
	} else {
		// Not yet mandatory but will be in 2024
		fmt.Printf("WARNING: environment variable %q is required (use k8s secret)", xelonClientIDEnv)
	}

	xelonClient := xelon.NewClient(token, opts...)

	tenant, _, err := xelonClient.Tenants.GetCurrent(context.Background())
	if err != nil {
		return nil, err
	}

	return &cloud{
		client:        xelonClient,
		loadbalancers: newLoadBalancers(xelonClient, tenant.TenantID, cloudID, clusterID),
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
