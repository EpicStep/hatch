package workload

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func testDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
			Annotations: map[string]string{
				"existing": "annotation",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx:1.25"},
						{Name: "sidecar", Image: "envoy:latest"},
					},
				},
			},
		},
	}
}

func testDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent",
			Namespace: "infra",
			Annotations: map[string]string{
				"team": "platform",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "agent"},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "agent:v2"},
					},
				},
			},
		},
	}
}

func testStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db",
			Namespace: "data",
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "db"},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "postgres", Image: "postgres:16"},
					},
				},
			},
		},
	}
}

func TestNew_Deployment(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testDeployment())

	w, err := New(context.Background(), client, "default", "deployment", "web")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"existing": "annotation"}, w.Annotations())
	assert.Equal(t, map[string]string{"app": "web"}, w.Selector())
	assert.Len(t, w.PodSpec().Containers, 2)
	assert.Equal(t, "nginx:1.25", w.PodSpec().Containers[0].Image)
}

func TestNew_DaemonSet(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testDaemonSet())

	w, err := New(context.Background(), client, "infra", "daemonset", "agent")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"team": "platform"}, w.Annotations())
	assert.Equal(t, map[string]string{"app": "agent"}, w.Selector())
	assert.Equal(t, "agent:v2", w.PodSpec().Containers[0].Image)
}

func TestNew_StatefulSet(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testStatefulSet())

	w, err := New(context.Background(), client, "data", "statefulset", "db")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"app": "db"}, w.Selector())
	assert.Equal(t, "postgres:16", w.PodSpec().Containers[0].Image)
}

func TestNew_UnsupportedKind(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset()

	_, err := New(context.Background(), client, "default", "job", "myjob")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported workload kind")
}

func TestNew_NotFound(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset()

	_, err := New(context.Background(), client, "default", "deployment", "nonexistent")
	require.Error(t, err)
}

func TestNew_CaseInsensitive(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testDeployment())

	w, err := New(context.Background(), client, "default", "Deployment", "web")
	require.NoError(t, err)
	assert.Equal(t, "nginx:1.25", w.PodSpec().Containers[0].Image)
}

func TestFindContainer(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testDeployment())

	w, err := New(context.Background(), client, "default", "deployment", "web")
	require.NoError(t, err)

	c, err := FindContainer(w, "app")
	require.NoError(t, err)
	assert.Equal(t, "nginx:1.25", c.Image)

	c, err = FindContainer(w, "sidecar")
	require.NoError(t, err)
	assert.Equal(t, "envoy:latest", c.Image)
}

func TestFindContainer_NotFound(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testDeployment())

	w, err := New(context.Background(), client, "default", "deployment", "web")
	require.NoError(t, err)

	_, err = FindContainer(w, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container 'missing' not found")
}

func TestDeepCopy_IsIndependent(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testDeployment())

	w, err := New(context.Background(), client, "default", "deployment", "web")
	require.NoError(t, err)

	snap := w.DeepCopy()

	// Mutate the original.
	w.PodSpec().Containers[0].Image = "mutated:latest"

	// Snapshot should be unaffected.
	snapDeploy, ok := snap.(*appsv1.Deployment)
	require.True(t, ok)
	assert.Equal(t, "nginx:1.25", snapDeploy.Spec.Template.Spec.Containers[0].Image)
}

func TestPatch_Deployment(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testDeployment())

	w, err := New(context.Background(), client, "default", "deployment", "web")
	require.NoError(t, err)

	patchData := []byte(`{"metadata":{"annotations":{"new":"value"}}}`)
	err = w.Patch(context.Background(), patchData, types.MergePatchType)
	require.NoError(t, err)
}

func TestSetAnnotations(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset(testDeployment())

	w, err := New(context.Background(), client, "default", "deployment", "web")
	require.NoError(t, err)

	w.SetAnnotations(map[string]string{"replaced": "true"})
	assert.Equal(t, map[string]string{"replaced": "true"}, w.Annotations())
}
