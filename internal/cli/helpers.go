package cli

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const (
	annotationActive        = "hatch.dev/active"
	annotationOriginalImage = "hatch.dev/original-image"
	annotationUser          = "hatch.dev/user"
	annotationNode          = "hatch.dev/node"
	annotationPod           = "hatch.dev/pod"
)

func kubectlRolloutStatus(ctx context.Context, flags *genericclioptions.ConfigFlags, namespace, kind, name string) error {
	args := append(kubectlGlobalArgs(flags), "rollout", "status", kind+"/"+name, "-n", namespace, "--timeout=120s")
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	return cmd.Run()
}

func kubectlPortForward(ctx context.Context, flags *genericclioptions.ConfigFlags, namespace, podName string, localPort, remotePort int) *exec.Cmd {
	args := append(kubectlGlobalArgs(flags), "port-forward", podName, fmt.Sprintf("%d:%d", localPort, remotePort), "-n", namespace)
	return exec.CommandContext(ctx, "kubectl", args...)
}

func waitForPodReady(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 120*time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return isPodReady(pod), nil
	})
}

func selectPod(ctx context.Context, clientset kubernetes.Interface, namespace string, selector map[string]string, nodeHint, podHint, storedNode, storedPod string) (*corev1.Pod, error) {
	podsClient := clientset.CoreV1().Pods(namespace)

	if podHint != "" {
		pod, err := podsClient.Get(ctx, podHint, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("pod %s not found: %w", podHint, err)
		}
		return pod, nil
	}

	if nodeHint != "" {
		return findPodOnNode(ctx, clientset, namespace, selector, nodeHint)
	}

	if storedPod != "" {
		pod, err := podsClient.Get(ctx, storedPod, metav1.GetOptions{})
		if err == nil {
			return pod, nil
		}
	}

	if storedNode != "" {
		pod, err := findPodOnNode(ctx, clientset, namespace, selector, storedNode)
		if err == nil {
			return pod, nil
		}
	}

	list, err := podsClient.List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(selector).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	if len(list.Items) == 0 {
		return nil, fmt.Errorf("no pods found matching selector %v", selector)
	}

	slices.SortFunc(list.Items, func(a, b corev1.Pod) int {
		return b.CreationTimestamp.Compare(a.CreationTimestamp.Time)
	})
	for i := range list.Items {
		if isPodReady(&list.Items[i]) {
			return &list.Items[i], nil
		}
	}
	return &list.Items[0], nil
}

func findPodOnNode(ctx context.Context, clientset kubernetes.Interface, namespace string, selector map[string]string, node string) (*corev1.Pod, error) {
	list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(selector).String(),
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", node).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on node %s: %w", node, err)
	}

	if len(list.Items) == 0 {
		return nil, fmt.Errorf("no pod found on node %s", node)
	}

	for _, pod := range list.Items {
		if !isPodReady(&pod) {
			continue
		}

		return &pod, nil
	}

	return &list.Items[0], nil
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.ObservedGeneration != pod.Generation {
		return false
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Type != corev1.PodReady {
			continue
		}

		return cond.Status == corev1.ConditionTrue
	}
	return false
}

func kubectlGlobalArgs(f *genericclioptions.ConfigFlags) []string {
	var args []string
	if f.KubeConfig != nil && *f.KubeConfig != "" {
		args = append(args, "--kubeconfig", *f.KubeConfig)
	}

	if f.Context != nil && *f.Context != "" {
		args = append(args, "--context", *f.Context)
	}

	return args
}
