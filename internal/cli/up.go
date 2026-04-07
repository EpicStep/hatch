package cli

import (
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

	sshPubKeyBytes, err := os.ReadFile(opts.sshKey)
	if err != nil {
		return fmt.Errorf("reading SSH key '%s': %w", opts.sshKey, err)
	}

	sshPubKey := strings.TrimSpace(string(sshPubKeyBytes))

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

	var storedNode, storedPod string
	reconnect := false
	if w.Annotations()[annotationActive] == "true" {
		activeUser := w.Annotations()[annotationUser]
		if activeUser != opts.user {
			return fmt.Errorf("dev mode already active (by %s), use 'hatch down' first or --user to match", activeUser)
		}

		fmt.Println("Dev mode already active, reconnecting...")
		storedNode = w.Annotations()[annotationNode]
		storedPod = w.Annotations()[annotationPod]

		reconnect = true
	}

	if !reconnect {
		originalImage := container.Image
		before := w.DeepCopy()

		// Typed mutations: set annotations, swap image, inject SSH key.
		ann := w.Annotations()
		if ann == nil {
			ann = make(map[string]string)
			w.SetAnnotations(ann)
		}
		ann[annotationActive] = "true"
		ann[annotationOriginalImage] = originalImage
		ann[annotationUser] = opts.user

		container.Image = opts.image
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

		if err = kubectlRolloutStatus(ctx, opts.k8sConfig, opts.namespace, opts.kind, opts.workload); err != nil {
			return fmt.Errorf("waiting for rollout: %w", err)
		}
	}

	pod, err := selectPod(ctx, client, opts.namespace, w.Selector(), opts.node, opts.pod, storedNode, storedPod)
	if err != nil {
		return fmt.Errorf("selecting pod: %w", err)
	}

	fmt.Printf("Waiting for pod %s...\n", pod.Name)
	if err = waitForPodReady(ctx, client, opts.namespace, pod.Name); err != nil {
		return fmt.Errorf("waiting for pod ready: %w", err)
	}

	// Record which pod we connected to for future reconnects.
	trackBefore := w.DeepCopy()
	ann := w.Annotations()
	if ann == nil {
		ann = make(map[string]string)
		w.SetAnnotations(ann)
	}

	ann[annotationPod] = pod.Name
	ann[annotationNode] = pod.Spec.NodeName

	trackPatch, err := workload.MergeFrom(trackBefore, w.Object())
	if err == nil {
		_ = w.Patch(ctx, trackPatch, types.MergePatchType)
	}

	// Remove stale host key to avoid MITM warning after container restart.
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	if err = knownhosts.RemoveEntry(knownHostsPath, "localhost", opts.localPort); err != nil {
		fmt.Printf("Warning: could not clean known_hosts: %v\n", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pfCmd := kubectlPortForward(ctx, opts.k8sConfig, opts.namespace, pod.Name, opts.localPort, 2222)
	pfCmd.Stdout = cmd.OutOrStdout()
	pfCmd.Stderr = cmd.OutOrStderr()

	if err = pfCmd.Start(); err != nil {
		return fmt.Errorf("starting port-forward: %w", err)
	}

	fmt.Println("\n=== Dev environment ready ===")
	fmt.Printf("SSH:   ssh -p %d nonroot@localhost\n", opts.localPort)
	fmt.Print("Stop:  Ctrl+C\n\n")

	if err = pfCmd.Wait(); err != nil {
		if ctx.Err() != nil {
			fmt.Println("\nStopping port-forward...")
			return nil
		}

		return fmt.Errorf("port-forward: %w", err)
	}

	return nil
}
