package xelon

import (
	"context"
	"fmt"
	"io"
	"os"

	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

const (
	ProviderName string = "xelon"

	xelonBaseURLEnv             string = "XELON_BASE_URL"
	xelonClientIDEnv            string = "XELON_CLIENT_ID"
	xelonCloudIDEnv             string = "XELON_CLOUD_ID"
	xelonKubernetesClusterIDEnv string = "XELON_KUBERNETES_CLUSTER_ID"
	xelonTokenEnv               string = "XELON_TOKEN"
)

type clients struct {
	k8s   kubernetes.Interface
	xelon *xelon.Client
}

type cloud struct {
	clients       *clients
	loadBalancers cloudprovider.LoadBalancer
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud()
	})
}

func newCloud() (cloudprovider.Interface, error) {
	klog.InfoS("Cloud controller manager information", "provider", ProviderName, "version_info", GetVersionInfo())

	token := os.Getenv(xelonTokenEnv)
	if token == "" {
		return nil, fmt.Errorf("environment variable %q is required (use k8s secret)", xelonTokenEnv)
	}

	cloudID := os.Getenv(xelonCloudIDEnv)
	if cloudID == "" {
		return nil, fmt.Errorf("environment variable %q is required", xelonCloudIDEnv)
	}

	clusterID := os.Getenv(xelonKubernetesClusterIDEnv)
	if clusterID == "" {
		return nil, fmt.Errorf("environment variable %q is required", xelonKubernetesClusterIDEnv)
	}

	opts := []xelon.ClientOption{xelon.WithUserAgent(UserAgent())}
	if apiURL := os.Getenv(xelonBaseURLEnv); apiURL != "" {
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

	clients := &clients{xelon: xelonClient}

	return &cloud{
		clients:       clients,
		loadBalancers: newLoadBalancers(clients, tenant.TenantID, cloudID, clusterID),
	}, nil
}

func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, _ <-chan struct{}) {
	config := clientBuilder.ConfigOrDie("xelon-cloud-controller-manager")
	c.clients.k8s = kubernetes.NewForConfigOrDie(config)
}

func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadBalancers, true
}

func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
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
