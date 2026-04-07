package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/EpicStep/hatch/internal/knownhosts"
	"github.com/EpicStep/hatch/internal/workload"
)

type upOptions struct {
	*generalOptions

	node      string
	pod       string
	sshKey    string
	localPort int
}

type reconnectHints struct {
	active bool
	node   string
	pod    string
}

func newUpCmd(generalOptions *generalOptions) *cobra.Command {
	opts := &upOptions{
		generalOptions: generalOptions,
	}

	home, _ := os.UserHomeDir()
	defaultKey := filepath.Join(home, ".ssh", "id_ed25519.pub")

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Swap workload image with dev container and start port-forwarding",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUp(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.node, "node", "", "select pod on a specific node (useful for DaemonSets)")
	cmd.Flags().StringVar(&opts.pod, "pod", "", "select a specific pod by name")
	cmd.Flags().StringVar(&opts.sshKey, "ssh-key", defaultKey, "path to SSH public key")
	cmd.Flags().IntVar(&opts.localPort, "local-port", 2222, "local port for SSH forwarding")

	return cmd
}

func runUp(cmd *cobra.Command, opts *upOptions) error {
	ctx := cmd.Context()

	if opts.node != "" && opts.pod != "" {
		return errors.New("--node and --pod are mutually exclusive")
	}

	if strings.ToLower(opts.kind) == "daemonset" && opts.node == "" && opts.pod == "" {
		fmt.Println("Warning: no --node specified for DaemonSet; will pick a pod from any node")
	}

	sshPubKey, err := readSSHPublicKey(opts.sshKey)
	if err != nil {
		return err
	}

	client, err := opts.kubeClient()
	if err != nil {
		return err
	}

	w, err := workload.New(ctx, client, opts.namespace, opts.kind, opts.workload)
	if err != nil {
		return err
	}

	container, err := workload.FindContainer(w, opts.container)
	if err != nil {
		return fmt.Errorf("in %s/%s: %w", opts.kind, opts.workload, err)
	}

	hints, err := checkReconnect(w, opts.user)
	if err != nil {
		return err
	}

	if !hints.active {
		if err = activateDevMode(ctx, opts, w, container, sshPubKey); err != nil {
			return err
		}
	}

	pod, err := selectPod(ctx, client, opts.namespace, w.Selector(), opts.node, opts.pod, hints)
	if err != nil {
		return fmt.Errorf("selecting pod: %w", err)
	}

	fmt.Printf("Waiting for pod %s...\n", pod.Name)

	if err = waitForPodReady(ctx, client, opts.namespace, pod.Name); err != nil {
		return fmt.Errorf("waiting for pod ready: %w", err)
	}

	if err = trackReconnectInfo(ctx, w, pod); err != nil {
		return err
	}

	cleanKnownHosts(opts.localPort)

	return startPortForward(cmd, opts, pod.Name)
}

func readSSHPublicKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading SSH key '%s': %w", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}

func checkReconnect(w workload.Workload, user string) (reconnectHints, error) {
	ann := w.Annotations()

	if ann[annotationActive] != "true" {
		return reconnectHints{}, nil
	}

	activeUser := ann[annotationUser]
	if activeUser != user {
		return reconnectHints{}, fmt.Errorf("dev mode already active (by %s), use 'hatch down' first or --user to match", activeUser)
	}

	fmt.Println("Dev mode already active, reconnecting...")

	return reconnectHints{
		active: true,
		node:   ann[annotationNode],
		pod:    ann[annotationPod],
	}, nil
}

func activateDevMode(ctx context.Context, opts *upOptions, w workload.Workload, container *corev1.Container, sshPubKey string) error {
	before := w.DeepCopy()

	ann := w.Annotations()
	if ann == nil {
		ann = make(map[string]string)
	}

	ann[annotationActive] = "true"
	ann[annotationOriginalImage] = container.Image
	ann[annotationUser] = opts.user

	if len(container.Command) > 0 {
		commandJSON, err := json.Marshal(container.Command)
		if err != nil {
			return fmt.Errorf("marshaling original command: %w", err)
		}

		ann[annotationOriginalCommand] = string(commandJSON)
	}

	if len(container.Args) > 0 {
		argsJSON, err := json.Marshal(container.Args)
		if err != nil {
			return fmt.Errorf("marshaling original args: %w", err)
		}

		ann[annotationOriginalArgs] = string(argsJSON)
	}

	w.SetAnnotations(ann)

	container.Image = opts.image
	container.Command = []string{"/home/nonroot/entrypoint-dev.sh"}
	container.Args = nil
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "AUTHORIZED_KEYS",
		Value: sshPubKey,
	})

	patchBytes, err := workload.MergeFrom(before, w.Object())
	if err != nil {
		return fmt.Errorf("building patch: %w", err)
	}

	fmt.Printf("Patching %s/%s: image=%s\n", opts.kind, opts.workload, opts.image)

	if err = w.Patch(ctx, patchBytes, types.StrategicMergePatchType); err != nil {
		return fmt.Errorf("patching workload: %w", err)
	}

	fmt.Println("Waiting for rollout...")

	return kubectlRolloutStatus(ctx, opts.k8sConfig, opts.namespace, opts.kind, opts.workload)
}

func trackReconnectInfo(ctx context.Context, w workload.Workload, pod *corev1.Pod) error {
	before := w.DeepCopy()

	ann := w.Annotations()
	if ann == nil {
		ann = make(map[string]string)
	}

	ann[annotationPod] = pod.Name
	ann[annotationNode] = pod.Spec.NodeName

	w.SetAnnotations(ann)

	patchBytes, err := workload.MergeFrom(before, w.Object())
	if err != nil {
		return err
	}

	return w.Patch(ctx, patchBytes, types.MergePatchType)
}

func cleanKnownHosts(localPort int) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Warning: could not clean known_hosts: %v\n", err)
		return
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	if err = knownhosts.RemoveEntry(knownHostsPath, "localhost", localPort); err != nil {
		fmt.Printf("Warning: could not clean known_hosts: %v\n", err)
	}
}

func startPortForward(cmd *cobra.Command, opts *upOptions, podName string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pfCmd := kubectlPortForward(ctx, opts.k8sConfig, opts.namespace, podName, opts.localPort, 2222)
	pfCmd.Stdout = cmd.OutOrStdout()
	pfCmd.Stderr = cmd.OutOrStderr()

	if err := pfCmd.Start(); err != nil {
		return fmt.Errorf("starting port-forward: %w", err)
	}

	fmt.Println("\n=== Dev environment ready ===")
	fmt.Printf("SSH:   ssh -p %d nonroot@localhost\n", opts.localPort)
	fmt.Print("Stop:  Ctrl+C\n\n")

	if err := pfCmd.Wait(); err != nil {
		if ctx.Err() != nil {
			fmt.Println("\nStopping port-forward...")
			return nil
		}

		return fmt.Errorf("port-forward: %w", err)
	}

	return nil
}
