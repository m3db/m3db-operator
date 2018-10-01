package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureService_Base(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	k8sops, err := newFakeK8sops()
	require.NoError(t, err)

	c := &Controller{
		k8sclient: k8sops,
	}

	err = c.ensureServices(nil)
	assert.Error(t, err)

	err = c.ensureServices(cluster)
	assert.NoError(t, err)

	for _, svcName := range []string{"m3dbnode-cluster-simple", "m3coordinator-cluster-simple"} {
		svc, err := k8sops.GetService(cluster, svcName)
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	}
}

func TestEnsureService_Custom(t *testing.T) {
	cluster := getFixture("cluster-services.yaml", t)
	k8sops, err := newFakeK8sops()
	require.NoError(t, err)

	c := &Controller{
		k8sclient: k8sops,
	}

	err = c.ensureServices(cluster)
	assert.NoError(t, err)

	for _, svcName := range []string{"m3dbnode-cluster-services", "custom-svc"} {
		svc, err := k8sops.GetService(cluster, svcName)
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	}

	_, err = k8sops.GetService(cluster, "m3coordinator-cluster-services")
	assert.Error(t, err)
}
