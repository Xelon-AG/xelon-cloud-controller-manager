package xelon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

func TestInstances_getNodeTypeFromControlPlaneNode(t *testing.T) {
	type testCase struct {
		input    *xelon.ClusterControlPlane
		expected string
	}
	tests := map[string]testCase{
		"nil": {
			input:    nil,
			expected: "",
		},
		"valid values": {
			input: &xelon.ClusterControlPlane{
				CPUCoreCount: 2,
				DiskSize:     50,
				Memory:       4,
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

func TestInstances_getNodeTypeFromClusterPool(t *testing.T) {
	type testCase struct {
		input    *xelon.ClusterPool
		expected string
	}
	tests := map[string]testCase{
		"nil": {
			input:    nil,
			expected: "",
		},
		"valid values": {
			input: &xelon.ClusterPool{
				CPUCoreCount: 2,
				DiskSize:     50,
				Memory:       4,
			},
			expected: "c2c-m4g-d50g",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := getNodeTypeFromClusterPool(test.input)
			assert.Equal(t, test.expected, actual)
		})
	}
}
