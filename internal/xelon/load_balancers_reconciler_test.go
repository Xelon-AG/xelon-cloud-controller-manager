package xelon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

func TestReconcile_createRules(t *testing.T) {
	type testCase struct {
		current  []xelon.LoadBalancerClusterForwardingRule
		desired  []xelon.LoadBalancerClusterForwardingRule
		expected []xelon.LoadBalancerClusterForwardingRule
	}
	tests := map[string]testCase{
		"nil": {
			current:  nil,
			desired:  nil,
			expected: nil,
		},
		"nil current": {
			current: nil,
			desired: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800},
			}},
			expected: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800},
			}},
		},
		"nil desired": {
			current: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800},
			}},
			desired:  nil,
			expected: nil,
		},
		"add rule from desired": {
			current: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080, ID: "5qggn9mtbz"},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800},
			}},
			desired: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800},
			}, {
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8090},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80900},
			}},
			expected: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8090},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80900},
			}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := reconcile(test.current, test.desired)
			assert.Equal(t, test.expected, actual.rulesToCreate)
		})
	}
}

func TestReconcile_updateRules(t *testing.T) {
	type testCase struct {
		current  []xelon.LoadBalancerClusterForwardingRule
		desired  []xelon.LoadBalancerClusterForwardingRule
		expected []xelon.LoadBalancerClusterForwardingRule
	}
	tests := map[string]testCase{
		"nil": {
			current:  nil,
			desired:  nil,
			expected: nil,
		},
		"update with new backend port": {
			current: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080, ID: "5qggn9mtbz"},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800, ID: "u0gkddw9rr"},
			}},
			desired: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 99999},
			}},
			expected: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080, ID: "5qggn9mtbz"},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 99999, ID: "u0gkddw9rr"},
			}},
		},
		"update with new proxy_protocol": {
			current: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080, ID: "5qggn9mtbz"},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800, ID: "u0gkddw9rr", ProxyProtocol: 0},
			}},
			desired: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800, ProxyProtocol: 1},
			}},
			expected: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080, ID: "5qggn9mtbz"},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800, ID: "u0gkddw9rr", ProxyProtocol: 1},
			}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := reconcile(test.current, test.desired)
			assert.Equal(t, test.expected, actual.rulesToUpdate)
		})
	}
}

func TestReconcile_deleteRules(t *testing.T) {
	type testCase struct {
		current  []xelon.LoadBalancerClusterForwardingRule
		desired  []xelon.LoadBalancerClusterForwardingRule
		expected []xelon.LoadBalancerClusterForwardingRule
	}
	tests := map[string]testCase{
		"nil": {
			current:  nil,
			desired:  nil,
			expected: nil,
		},
		"remove non-used existed rule": {
			current: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080, ID: "5qggn9mtbz"},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800},
			}},
			desired: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8090},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80900},
			}},
			expected: []xelon.LoadBalancerClusterForwardingRule{{
				Frontend: &xelon.LoadBalancerClusterForwardingRuleFrontendConfiguration{Port: 8080, ID: "5qggn9mtbz"},
				Backend:  &xelon.LoadBalancerClusterForwardingRuleBackendConfiguration{Port: 80800},
			}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := reconcile(test.current, test.desired)
			assert.Equal(t, test.expected, actual.rulesToDelete)
		})
	}
}
