package dratopology

import (
	"context"
	"fmt"

	resourcev1 "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	driverName      = "topo.k8s-sigs.io"
	driverMatchExpr = fmt.Sprintf("device.driver == '%s'", driverName)
)

// SyncDeviceClasses ensures that the DeviceClass objects in the API server
// reflect the state of the BasicHierarchy.
func SyncDeviceClasses(ctx context.Context, clientset kubernetes.Interface, hierarchy *BasicHierarchy) error {
	logger := klog.FromContext(ctx)

	if len(hierarchy.selectors) != len(hierarchy.levels) {
		return fmt.Errorf("hierarchy selectors and levels have different lengths")
	}

	for _, level := range hierarchy.levels {
		topoLevel := level.Name
		name := fmt.Sprintf("%s.%s", topoLevel, driverName)

		logger.V(4).Info("Processing device class", "name", name, "level", level)

		desiredDeviceClass, err := buildDeviceClass(name, topoLevel)
		if err != nil {
			return fmt.Errorf("failed to build desired DeviceClass for level %s: %w", level, err)
		}

		currentDeviceClass, err := clientset.ResourceV1().DeviceClasses().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get DeviceClass %s: %w", name, err)
			}
			// DeviceClass does not exist, create it.
			_, err := clientset.ResourceV1().DeviceClasses().Create(ctx, desiredDeviceClass, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create DeviceClass %s: %w", name, err)
			}
			logger.V(2).Info("Created DeviceClass", "name", name)
			continue
		}

		// DeviceClass exists, check if update is needed.
		// For now, a simple overwrite.
		// Today topo DeviceClasses are very simple as long as the topoLevel and driverNames stay constant.
		currentDeviceClass.Spec = desiredDeviceClass.Spec
		_, err = clientset.ResourceV1().DeviceClasses().Update(ctx, currentDeviceClass, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update DeviceClass %s: %w", name, err)
		}
		logger.V(4).Info("Updated DeviceClass", "name", name)
	}

	return nil
}

func buildDeviceClass(name string, topoLevel string) (*resourcev1.DeviceClass, error) {

	expression := fmt.Sprintf("device.attributes['%s'].topoLevel == '%s'", driverName, topoLevel)

	deviceSelectors := []resourcev1.DeviceSelector{
		{
			CEL: &resourcev1.CELDeviceSelector{
				Expression: expression,
			},
		},
		{
			CEL: &resourcev1.CELDeviceSelector{
				Expression: driverMatchExpr,
			},
		},
	}

	return &resourcev1.DeviceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: resourcev1.DeviceClassSpec{
			Selectors: deviceSelectors,
		},
	}, nil
}
