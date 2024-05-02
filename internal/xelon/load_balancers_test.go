package xelon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

func TestIsVirtualIPAvailable_emptyVirtualIP(t *testing.T) {
	available := isVirtualIPAvailable(nil, nil, &v1.Service{})

	assert.Equal(t, false, available)
}

func TestIsVirtualIPAvailable_emptyService(t *testing.T) {
	available := isVirtualIPAvailable(&xelon.LoadBalancerClusterVirtualIP{}, nil, nil)

	assert.Equal(t, false, available)
}

func TestIsVirtualIPAvailable_reservedState(t *testing.T) {
	virtualIP := &xelon.LoadBalancerClusterVirtualIP{State: "reserved"}

	available := isVirtualIPAvailable(virtualIP, nil, nil)

	assert.Equal(t, false, available)
}

func TestIsVirtualIPAvailable_noFrontendForwardingRules(t *testing.T) {
	virtualIP := &xelon.LoadBalancerClusterVirtualIP{State: "free"}
	forwardingRules := []xelon.LoadBalancerClusterForwardingRule{
		{Backend: &xelon.LoadBalancerClusterForwardingRuleConfiguration{Port: 8080}},
		{Backend: &xelon.LoadBalancerClusterForwardingRuleConfiguration{Port: 8081}},
		{Backend: &xelon.LoadBalancerClusterForwardingRuleConfiguration{Port: 8082}},
	}
	service := &v1.Service{Spec: v1.ServiceSpec{
		Ports: []v1.ServicePort{
			{Port: 8080},
			{Port: 8081},
			{Port: 8082},
		},
	}}

	available := isVirtualIPAvailable(virtualIP, forwardingRules, service)

	assert.Equal(t, true, available)
}

func TestIsVirtualIPAvailable_frontedPortExists(t *testing.T) {
	virtualIP := &xelon.LoadBalancerClusterVirtualIP{State: "free"}
	forwardingRules := []xelon.LoadBalancerClusterForwardingRule{
		{Frontend: &xelon.LoadBalancerClusterForwardingRuleConfiguration{Port: 8080}},
	}
	service := &v1.Service{Spec: v1.ServiceSpec{
		Ports: []v1.ServicePort{
			{Port: 8080},
			{Port: 8081},
			{Port: 8082},
		},
	}}

	available := isVirtualIPAvailable(virtualIP, forwardingRules, service)

	assert.Equal(t, false, available)
}

func TestIsVirtualIPAvailable_frontedPortAvailable(t *testing.T) {
	virtualIP := &xelon.LoadBalancerClusterVirtualIP{State: "free"}
	forwardingRules := []xelon.LoadBalancerClusterForwardingRule{
		{Frontend: &xelon.LoadBalancerClusterForwardingRuleConfiguration{Port: 8080}},
	}
	service := &v1.Service{Spec: v1.ServiceSpec{
		Ports: []v1.ServicePort{
			{Port: 9090},
		},
	}}

	available := isVirtualIPAvailable(virtualIP, forwardingRules, service)

	assert.Equal(t, true, available)
}
