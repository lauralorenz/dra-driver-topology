package dratopology

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSyncDeviceClasses(t *testing.T) {
	ctx := context.Background()

	// Setup hierarchy data
	requirement, err := labels.NewRequirement("topo.example.com/host", "exists", nil)
	require.NoError(t, err)
	selector := labels.NewSelector().Add(*requirement)

	hierarchy := &BasicHeirarchy{
		levels:    []Level{{Name: "host", Label: "topo.example.com/host"}},
		selectors: []labels.Selector{selector},
		count:     1,
	}

	requirement2, err := labels.NewRequirement("another-label", "exists", nil)
	require.NoError(t, err)
	selector2 := labels.NewSelector().Add(*requirement, *requirement2)

	hierarchy2 := &BasicHeirarchy{
		levels:    []Level{{Name: "host", Label: "topo.example.com/updated-host-label"}},
		selectors: []labels.Selector{selector2},
		count:     1,
	}

	// Fake clientset
	clientset := fake.NewClientset()

	// First sync, should create the DeviceClass
	err = SyncDeviceClasses(ctx, clientset, hierarchy)
	require.NoError(t, err)

	// Verify expected DeviceClass was created
	dcName := "host.topo.k8s-sigs.io"
	dc, err := clientset.ResourceV1().DeviceClasses().Get(ctx, dcName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, dcName, dc.Name)
	// Expect 2 CEL selectors: one to match the driver, one to match the topoLevel
	require.Len(t, dc.Spec.Selectors, 2)
	require.NotNil(t, dc.Spec.Selectors[0].CEL)
	assert.Equal(t, "device.attributes['topo.k8s-sigs.io'].topoLevel == 'host'", dc.Spec.Selectors[0].CEL.Expression)
	require.NotNil(t, dc.Spec.Selectors[1].CEL)
	assert.Equal(t, "device.driver == 'topo.k8s-sigs.io'", dc.Spec.Selectors[1].CEL.Expression)

	// Second sync, should overwrite the DeviceClass selectors
	// But today we do not have anything meaningful to overwrite
	// So it ends up being a noop
	currGeneration := dc.Generation
	err = SyncDeviceClasses(ctx, clientset, hierarchy2)
	require.NoError(t, err)
	assert.Equal(t, currGeneration, dc.Generation)
}

func TestSyncDeviceClasses_NewClass(t *testing.T) {
	ctx := context.Background()

	// Setup hierarchy data
	req1, err := labels.NewRequirement("topo.example.com/host", "exists", nil)
	require.NoError(t, err)
	sel1 := labels.NewSelector().Add(*req1)

	req2, err := labels.NewRequirement("another-label", "exists", nil)
	require.NoError(t, err)
	sel2 := labels.NewSelector().Add(*req2)

	hierarchy := &BasicHeirarchy{
		levels: []Level{
			{Name: "host", Label: "topo.example.com/host"},
			{Name: "subblock", Label: "another-label"},
		},
		selectors: []labels.Selector{sel1, sel2},
		count:     2,
	}

	// Fake clientset
	clientset := fake.NewClientset()

	// Sync, should create two DeviceClasses
	err = SyncDeviceClasses(ctx, clientset, hierarchy)
	require.NoError(t, err)

	// Verify DeviceClasses were created
	dcs, err := clientset.ResourceV1().DeviceClasses().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, dcs.Items, 2)

	// Check first class
	dc1Name := "host.topo.k8s-sigs.io"
	dc1, err := clientset.ResourceV1().DeviceClasses().Get(ctx, dc1Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, dc1Name, dc1.Name)
	require.Len(t, dc1.Spec.Selectors, 2)
	require.NotNil(t, dc1.Spec.Selectors[0].CEL)
	assert.Equal(t, "device.attributes['topo.k8s-sigs.io'].topoLevel == 'host'", dc1.Spec.Selectors[0].CEL.Expression)
	require.NotNil(t, dc1.Spec.Selectors[1].CEL)
	assert.Equal(t, "device.driver == 'topo.k8s-sigs.io'", dc1.Spec.Selectors[1].CEL.Expression)

	// Check second class
	dc2Name := "subblock.topo.k8s-sigs.io"
	dc2, err := clientset.ResourceV1().DeviceClasses().Get(ctx, dc2Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, dc2Name, dc2.Name)
	require.Len(t, dc2.Spec.Selectors, 2)
	require.NotNil(t, dc2.Spec.Selectors[0].CEL)
	assert.Equal(t, "device.attributes['topo.k8s-sigs.io'].topoLevel == 'subblock'", dc2.Spec.Selectors[0].CEL.Expression)
	require.NotNil(t, dc2.Spec.Selectors[1].CEL)
	assert.Equal(t, "device.driver == 'topo.k8s-sigs.io'", dc2.Spec.Selectors[1].CEL.Expression)
}

func TestBuildDeviceClass(t *testing.T) {

	dcIn, err := buildDeviceClass("block.topo.k8s-sigs.io", "block")
	require.NoError(t, err)
	assert.Equal(t, "block.topo.k8s-sigs.io", dcIn.Name)
	assert.Equal(t, "device.attributes['topo.k8s-sigs.io'].topoLevel == 'block'", dcIn.Spec.Selectors[0].CEL.Expression)
	assert.Equal(t, "device.driver == 'topo.k8s-sigs.io'", dcIn.Spec.Selectors[1].CEL.Expression)
}
