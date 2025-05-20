package xelon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

func TestLoadBalancers_buildLoadBalancerStatusIngress(t *testing.T) {
	vipIPMode := v1.LoadBalancerIPModeVIP
	proxyIPMode := v1.LoadBalancerIPModeProxy
	type testCase struct {
		inputLB  *xelonLoadBalancer
		inputSVC *v1.Service
		expected []v1.LoadBalancerIngress
	}
	tests := map[string]testCase{
		"default": {
			inputLB:  &xelonLoadBalancer{},
			inputSVC: &v1.Service{Spec: v1.ServiceSpec{}},
			expected: []v1.LoadBalancerIngress{{
				IPMode: &vipIPMode,
			}},
		},
		"proxy protocol 0": {
			inputLB: &xelonLoadBalancer{},
			inputSVC: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"service.beta.kubernetes.io/xelon-load-balancer-cluster-proxy-protocol-version": "0"},
				},
				Spec: v1.ServiceSpec{},
			},
			expected: []v1.LoadBalancerIngress{{
				IPMode: &vipIPMode,
			}},
		},
		"proxy protocol 1": {
			inputLB: &xelonLoadBalancer{},
			inputSVC: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"service.beta.kubernetes.io/xelon-load-balancer-cluster-proxy-protocol-version": "1"},
				},
				Spec: v1.ServiceSpec{},
			},
			expected: []v1.LoadBalancerIngress{{
				IPMode: &proxyIPMode,
			}},
		},
		"proxy protocol 2": {
			inputLB: &xelonLoadBalancer{},
			inputSVC: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"service.beta.kubernetes.io/xelon-load-balancer-cluster-proxy-protocol-version": "2"},
				},
				Spec: v1.ServiceSpec{},
			},
			expected: []v1.LoadBalancerIngress{{
				IPMode: &proxyIPMode,
			}},
		},
		"invalid proxy protocol": {
			inputLB: &xelonLoadBalancer{},
			inputSVC: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"service.beta.kubernetes.io/xelon-load-balancer-cluster-proxy-protocol-version": "invalid"},
				},
				Spec: v1.ServiceSpec{},
			},
			expected: []v1.LoadBalancerIngress{{
				IPMode: &vipIPMode,
			}},
		},
	}

	l := &loadBalancers{}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := l.buildLoadBalancerStatusIngress(context.TODO(), test.inputLB, test.inputSVC)
			assert.Equal(t, test.expected, actual)
		})
	}
}

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
		{Backend: &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 8080}},
		{Backend: &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 8081}},
		{Backend: &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 8082}},
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
		{Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080}},
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
		{Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080}},
	}
	service := &v1.Service{Spec: v1.ServiceSpec{
		Ports: []v1.ServicePort{
			{Port: 9090},
		},
	}}

	available := isVirtualIPAvailable(virtualIP, forwardingRules, service)

	assert.Equal(t, true, available)
}
