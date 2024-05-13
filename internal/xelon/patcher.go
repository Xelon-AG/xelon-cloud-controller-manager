package xelon

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

type servicePatcher struct {
	k8sClient kubernetes.Interface
	current   *v1.Service
	modified  *v1.Service
}

func newServicePatcher(k8sClient kubernetes.Interface, service *v1.Service) servicePatcher {
	return servicePatcher{
		k8sClient: k8sClient,
		current:   service.DeepCopy(),
		modified:  service,
	}
}

func (p *servicePatcher) Patch(ctx context.Context) error {
	currentJSON, err := json.Marshal(p.current)
	if err != nil {
		return fmt.Errorf("failed to serialize current service object: %s", err)
	}

	modifiedJSON, err := json.Marshal(p.modified)
	if err != nil {
		return fmt.Errorf("failed to serialize modified service object: %s", err)
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(currentJSON, modifiedJSON, v1.Service{})
	if err != nil {
		return fmt.Errorf("failed to create 2-way merge patch: %s", err)
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return nil
	}
	_, err = p.k8sClient.CoreV1().Services(p.current.Namespace).Patch(ctx, p.current.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch service object %s/%s: %s", p.current.Namespace, p.current.Name, err)
	}

	return nil
}
