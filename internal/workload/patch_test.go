package workload

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// patchPath navigates a JSON object by keys, returning the value at the end.
func patchPath(t *testing.T, data []byte, keys ...string) any {
	t.Helper()
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	var cur any = raw
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		require.True(t, ok, "expected map at key %q, got %T", k, cur)
		cur = m[k]
	}
	return cur
}

func TestMergeFrom_ImageChange(t *testing.T) {
	t.Parallel()
	deploy := testDeployment()
	before := deploy.DeepCopy()

	deploy.Spec.Template.Spec.Containers[0].Image = "devimage:latest"

	patch, err := MergeFrom(before, deploy)
	require.NoError(t, err)

	containers, ok := patchPath(t, patch, "spec", "template", "spec", "containers").([]any)
	require.True(t, ok)
	require.Len(t, containers, 1)

	c, ok := containers[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "app", c["name"])
	assert.Equal(t, "devimage:latest", c["image"])
}

func TestMergeFrom_AnnotationAdd(t *testing.T) {
	t.Parallel()
	deploy := testDeployment()
	before := deploy.DeepCopy()

	deploy.Annotations["hatch.dev/active"] = "true"
	deploy.Annotations["hatch.dev/user"] = "alice"

	patch, err := MergeFrom(before, deploy)
	require.NoError(t, err)

	ann, ok := patchPath(t, patch, "metadata", "annotations").(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "true", ann["hatch.dev/active"])
	assert.Equal(t, "alice", ann["hatch.dev/user"])
	// Existing annotation should NOT appear in the patch (unchanged).
	_, hasExisting := ann["existing"]
	assert.False(t, hasExisting)
}

func TestMergeFrom_AnnotationRemoval(t *testing.T) {
	t.Parallel()
	deploy := testDeployment()
	deploy.Annotations["hatch.dev/active"] = "true"
	deploy.Annotations["hatch.dev/user"] = "alice"
	before := deploy.DeepCopy()

	delete(deploy.Annotations, "hatch.dev/active")
	delete(deploy.Annotations, "hatch.dev/user")

	patch, err := MergeFrom(before, deploy)
	require.NoError(t, err)

	ann, ok := patchPath(t, patch, "metadata", "annotations").(map[string]any)
	require.True(t, ok)
	// Deleted annotations become null in the strategic merge patch.
	assert.Nil(t, ann["hatch.dev/active"])
	assert.Nil(t, ann["hatch.dev/user"])
}

func TestMergeFrom_EnvVarAdd(t *testing.T) {
	t.Parallel()
	deploy := testDeployment()
	before := deploy.DeepCopy()

	deploy.Spec.Template.Spec.Containers[0].Env = append(
		deploy.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "AUTHORIZED_KEYS", Value: "ssh-ed25519 AAAA..."},
	)

	patch, err := MergeFrom(before, deploy)
	require.NoError(t, err)

	containers, ok := patchPath(t, patch, "spec", "template", "spec", "containers").([]any)
	require.True(t, ok)

	c, ok := containers[0].(map[string]any)
	require.True(t, ok)

	envs, ok := c["env"].([]any)
	require.True(t, ok)
	require.Len(t, envs, 1)

	env, ok := envs[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AUTHORIZED_KEYS", env["name"])
}

func TestMergeFrom_NoDiff(t *testing.T) {
	t.Parallel()
	deploy := testDeployment()
	before := deploy.DeepCopy()

	patch, err := MergeFrom(before, deploy)
	require.NoError(t, err)

	assert.JSONEq(t, `{}`, string(patch))
}

func TestMergeFrom_RoundTrip(t *testing.T) {
	t.Parallel()
	deploy := testDeployment()
	before := deploy.DeepCopy()

	// Apply multiple mutations.
	deploy.Annotations["hatch.dev/active"] = "true"
	deploy.Spec.Template.Spec.Containers[0].Image = "devimage:latest"
	deploy.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "AUTHORIZED_KEYS", Value: "ssh-key"},
	}

	patch, err := MergeFrom(before, deploy)
	require.NoError(t, err)

	// Apply patch to the original to verify round-trip.
	originalJSON, err := json.Marshal(before)
	require.NoError(t, err)

	result, err := strategicpatch.StrategicMergePatch(originalJSON, patch, &appsv1.Deployment{})
	require.NoError(t, err)

	var restored appsv1.Deployment
	require.NoError(t, json.Unmarshal(result, &restored))

	assert.Equal(t, "true", restored.Annotations["hatch.dev/active"])
	assert.Equal(t, "annotation", restored.Annotations["existing"])
	assert.Equal(t, "devimage:latest", restored.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "envoy:latest", restored.Spec.Template.Spec.Containers[1].Image)
	require.Len(t, restored.Spec.Template.Spec.Containers[0].Env, 1)
	assert.Equal(t, "AUTHORIZED_KEYS", restored.Spec.Template.Spec.Containers[0].Env[0].Name)
}
