package workload

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// Workload abstracts DaemonSet, Deployment, and StatefulSet so callers
// never switch on kind.
type Workload interface {
	// Annotations returns the workload-level metadata annotations (mutable).
	Annotations() map[string]string
	// SetAnnotations replaces the annotation map on the workload object.
	SetAnnotations(map[string]string)
	// Selector returns the pod-template match labels.
	Selector() map[string]string
	// PodSpec returns the pod-template spec (mutable).
	PodSpec() *corev1.PodSpec
	// Object returns the current typed object.
	Object() runtime.Object
	DeepCopy() runtime.Object
	// Patch applies raw patch bytes to the API server.
	Patch(ctx context.Context, data []byte, pt types.PatchType) error
}

// New fetches the named workload from the API server and returns a Workload adapter.
func New(ctx context.Context, client kubernetes.Interface, namespace, kind, name string) (Workload, error) {
	opts := metav1.GetOptions{}
	switch strings.ToLower(kind) {
	case "daemonset":
		obj, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}
		return &daemonSetWorkload{obj: obj, client: client, namespace: namespace}, nil
	case "deployment":
		obj, err := client.AppsV1().Deployments(namespace).Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}
		return &deploymentWorkload{obj: obj, client: client, namespace: namespace}, nil
	case "statefulset":
		obj, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}
		return &statefulSetWorkload{obj: obj, client: client, namespace: namespace}, nil
	default:
		return nil, fmt.Errorf("unsupported workload kind: %s (use daemonset, deployment, or statefulset)", kind)
	}
}

// FindContainer returns a pointer to the named container in the pod spec.
func FindContainer(w Workload, name string) (corev1.Container, error) {
	for _, container := range w.PodSpec().Containers {
		if container.Name != name {
			continue
		}

		return container, nil
	}

	return corev1.Container{}, fmt.Errorf("container '%s' not found", name)
}

type daemonSetWorkload struct {
	obj       *appsv1.DaemonSet
	client    kubernetes.Interface
	namespace string
}

func (w *daemonSetWorkload) Annotations() map[string]string     { return w.obj.Annotations }
func (w *daemonSetWorkload) SetAnnotations(a map[string]string) { w.obj.Annotations = a }
func (w *daemonSetWorkload) Selector() map[string]string        { return w.obj.Spec.Selector.MatchLabels }
func (w *daemonSetWorkload) PodSpec() *corev1.PodSpec           { return &w.obj.Spec.Template.Spec }
func (w *daemonSetWorkload) Object() runtime.Object             { return w.obj }
func (w *daemonSetWorkload) DeepCopy() runtime.Object           { return w.obj.DeepCopy() }

func (w *daemonSetWorkload) Patch(ctx context.Context, data []byte, pt types.PatchType) error {
	_, err := w.client.AppsV1().DaemonSets(w.namespace).Patch(ctx, w.obj.Name, pt, data, metav1.PatchOptions{})
	return err
}

type deploymentWorkload struct {
	obj       *appsv1.Deployment
	client    kubernetes.Interface
	namespace string
}

func (w *deploymentWorkload) Annotations() map[string]string     { return w.obj.Annotations }
func (w *deploymentWorkload) SetAnnotations(a map[string]string) { w.obj.Annotations = a }
func (w *deploymentWorkload) Selector() map[string]string        { return w.obj.Spec.Selector.MatchLabels }
func (w *deploymentWorkload) PodSpec() *corev1.PodSpec           { return &w.obj.Spec.Template.Spec }
func (w *deploymentWorkload) Object() runtime.Object             { return w.obj }
func (w *deploymentWorkload) DeepCopy() runtime.Object           { return w.obj.DeepCopy() }

func (w *deploymentWorkload) Patch(ctx context.Context, data []byte, pt types.PatchType) error {
	_, err := w.client.AppsV1().Deployments(w.namespace).Patch(ctx, w.obj.Name, pt, data, metav1.PatchOptions{})
	return err
}

type statefulSetWorkload struct {
	obj       *appsv1.StatefulSet
	client    kubernetes.Interface
	namespace string
}

func (w *statefulSetWorkload) Annotations() map[string]string     { return w.obj.Annotations }
func (w *statefulSetWorkload) SetAnnotations(a map[string]string) { w.obj.Annotations = a }
func (w *statefulSetWorkload) Selector() map[string]string        { return w.obj.Spec.Selector.MatchLabels }
func (w *statefulSetWorkload) PodSpec() *corev1.PodSpec           { return &w.obj.Spec.Template.Spec }
func (w *statefulSetWorkload) Object() runtime.Object             { return w.obj }
func (w *statefulSetWorkload) DeepCopy() runtime.Object           { return w.obj.DeepCopy() }

func (w *statefulSetWorkload) Patch(ctx context.Context, data []byte, pt types.PatchType) error {
	_, err := w.client.AppsV1().StatefulSets(w.namespace).Patch(ctx, w.obj.Name, pt, data, metav1.PatchOptions{})
	return err
}
