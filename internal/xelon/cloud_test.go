package xelon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloudProviderCapabilities(t *testing.T) {
	c := &cloud{
		instances:     &instances{},
		loadBalancers: &loadBalancers{},
	}

	assert.Equal(t, ProviderName, c.ProviderName())
	assert.False(t, c.HasClusterID())

	instancesV2, supported := c.InstancesV2()
	assert.True(t, supported)
	assert.NotNil(t, instancesV2)

	loadBalancer, supported := c.LoadBalancer()
	assert.True(t, supported)
	assert.NotNil(t, loadBalancer)

	instancesV1, supported := c.Instances()
	assert.False(t, supported)
	assert.Nil(t, instancesV1)

	zones, supported := c.Zones()
	assert.False(t, supported)
	assert.Nil(t, zones)

	clusters, supported := c.Clusters()
	assert.False(t, supported)
	assert.Nil(t, clusters)

	routes, supported := c.Routes()
	assert.False(t, supported)
	assert.Nil(t, routes)
}
