package xelon

import (
	"context"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

type loadBalancers struct {
	client *xelon.Client
}

func newLoadBalancers(client *xelon.Client) cloudprovider.LoadBalancer {
	return &loadBalancers{client: client}
}

func (l loadBalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	panic("implement me")
}

func (l loadBalancers) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	panic("implement me")
}

func (l loadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	panic("implement me")
}

func (l loadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	panic("implement me")
}

func (l loadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	panic("implement me")
}
