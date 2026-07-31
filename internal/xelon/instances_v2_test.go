package xelon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

func TestInstances_refreshNodes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /kubernetes/cluster-id/control-planes", func(w http.ResponseWriter, _ *http.Request) {
		controlPlane := xelon.KubernetesClusterControlPlane{
			CPUCores: 2,
			DiskSize: 50,
			RAM:      4,
			Nodes: []xelon.KubernetesClusterNode{{
				LocalVMID: "control-plane-vm-id",
				Name:      "control-plane-1",
			}},
		}
		assert.NoError(t, json.NewEncoder(w).Encode(controlPlane))
	})
	mux.HandleFunc("GET /kubernetes/cluster-id/pools", func(w http.ResponseWriter, _ *http.Request) {
		nodePools := []xelon.KubernetesClusterNodePool{{
			CPUCores: 4,
			DiskSize: 100,
			RAM:      8,
			Nodes: []xelon.KubernetesClusterNode{{
				LocalVMID: "worker-vm-id",
				Name:      "worker-1",
			}},
		}}
		assert.NoError(t, json.NewEncoder(w).Encode(nodePools))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	xelonClient := xelon.NewClient("token", xelon.WithBaseURL(server.URL+"/"))
	i := &instances{
		client:    &clients{xelon: xelonClient},
		clusterID: "cluster-id",
		ttl:       15 * time.Second,
	}

	err := i.refreshNodes(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, []xelonNode{
		{localVMID: "control-plane-vm-id", name: "control-plane-1", nodeType: "c2c-m4g-d50g"},
		{localVMID: "worker-vm-id", name: "worker-1", nodeType: "c4c-m8g-d100g"},
	}, i.nodes)
}

func TestInstances_getNodeTypeFromControlPlaneNode(t *testing.T) {
	type testCase struct {
		input    *xelon.KubernetesClusterControlPlane
		expected string
	}
	tests := map[string]testCase{
		"nil": {
			input:    nil,
			expected: "",
		},
		"valid values": {
			input: &xelon.KubernetesClusterControlPlane{
				CPUCores: 2,
				DiskSize: 50,
				RAM:      4,
			},
			expected: "c2c-m4g-d50g",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := getNodeTypeFromControlPlaneNode(test.input)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestInstances_getNodeTypeFromNodePool(t *testing.T) {
	type testCase struct {
		input    *xelon.KubernetesClusterNodePool
		expected string
	}
	tests := map[string]testCase{
		"nil": {
			input:    nil,
			expected: "",
		},
		"valid values": {
			input: &xelon.KubernetesClusterNodePool{
				CPUCores: 2,
				DiskSize: 50,
				RAM:      4,
			},
			expected: "c2c-m4g-d50g",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := getNodeTypeFromNodePool(test.input)
			assert.Equal(t, test.expected, actual)
		})
	}
}
