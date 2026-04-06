package cli

import (
	"context"
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
	*hatchOptions

	Node      string
	Pod       string
	SSHKey    string
	LocalPort int
}

func newUpCmd(ho *hatchOptions) *cobra.Command {
	o := &upOptions{hatchOptions: ho}

	home, _ := os.UserHomeDir()
	defaultKey := filepath.Join(home, ".ssh", "id_ed25519.pub")

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Swap workload image with dev container and start port-forwarding",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&o.Node, "node", "", "select pod on a specific node (useful for DaemonSets)")
	cmd.Flags().StringVar(&o.Pod, "pod", "", "select a specific pod by name")
	cmd.Flags().StringVar(&o.SSHKey, "ssh-key", defaultKey, "path to SSH public key")
	cmd.Flags().IntVar(&o.LocalPort, "local-port", 2222, "local port for SSH forwarding")

	return cmd
}

func (o *upOptions) Run(ctx context.Context) error {
	cfg := o.config

	if err := o.validate(); err != nil {
		return err
	}

	if o.Node != "" && o.Pod != "" {
		return errors.New("--node and --pod are mutually exclusive")
	}

	if strings.ToLower(cfg.Kind) == "daemonset" && o.Node == "" && o.Pod == "" {
		fmt.Fprintln(o.streams.ErrOut, "Warning: no --node specified for DaemonSet; will pick a pod from any node")
	}

	sshPubKeyBytes, err := os.ReadFile(o.SSHKey)
	if err != nil {
		return fmt.Errorf("reading SSH key %s: %w", o.SSHKey, err)
	}

	sshPubKey := strings.TrimSpace(string(sshPubKeyBytes))

	clientset, err := o.kubeClient()
	if err != nil {
		return err
	}

	namespace := cfg.Namespace

	w, err := workload.New(ctx, clientset, namespace, cfg.Kind, cfg.Workload)
	if err != nil {
		return err
	}

	container, _, err := workload.FindContainer(w, cfg.Container)
	if err != nil {
		return fmt.Errorf("in %s/%s: %w", cfg.Kind, cfg.Workload, err)
	}

	var storedNode, storedPod string
	reconnect := false
	if w.Annotations()[annotationActive] == "true" {
		activeUser := w.Annotations()[annotationUser]
		if activeUser != o.user {
			return fmt.Errorf("dev mode already active (by %s), use 'hatch down' first or --user to match", activeUser)
		}
		fmt.Fprintln(o.streams.Out, "Dev mode already active, reconnecting...")
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
		ann[annotationUser] = o.user

		container.Image = cfg.Image
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "AUTHORIZED_KEYS",
			Value: sshPubKey,
		})

		patchBytes, err := workload.MergeFrom(before, w.Object())
		if err != nil {
			return fmt.Errorf("building patch: %w", err)
		}

		fmt.Fprintf(o.streams.Out, "Patching %s/%s: image=%s\n", cfg.Kind, cfg.Workload, cfg.Image)

		if err := w.Patch(ctx, patchBytes, types.StrategicMergePatchType); err != nil {
			return fmt.Errorf("patching workload: %w", err)
		}

		fmt.Fprintln(o.streams.Out, "Waiting for rollout...")

		if err := kubectlRolloutStatus(ctx, o.configFlags, namespace, cfg.Kind, cfg.Workload); err != nil {
			return fmt.Errorf("waiting for rollout: %w", err)
		}
	}

	pod, err := selectPod(ctx, clientset, namespace, w.Selector(), o.Node, o.Pod, storedNode, storedPod)
	if err != nil {
		return fmt.Errorf("selecting pod: %w", err)
	}

	fmt.Fprintf(o.streams.Out, "Waiting for pod %s...\n", pod.Name)

	if err := waitForPodReady(ctx, clientset, namespace, pod.Name); err != nil {
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
	home, _ := os.UserHomeDir()
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	if err := knownhosts.RemoveEntry(knownHostsPath, "localhost", o.LocalPort); err != nil {
		fmt.Fprintf(o.streams.ErrOut, "Warning: could not clean known_hosts: %v\n", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pfCmd := kubectlPortForward(ctx, o.configFlags, namespace, pod.Name, o.LocalPort, 2222)
	pfCmd.Stdout = o.streams.Out
	pfCmd.Stderr = o.streams.ErrOut

	if err := pfCmd.Start(); err != nil {
		return fmt.Errorf("starting port-forward: %w", err)
	}

	fmt.Fprint(o.streams.Out, "\n=== Dev environment ready ===\n")
	fmt.Fprintf(o.streams.Out, "SSH:   ssh -p %d nonroot@localhost\n", o.LocalPort)
	fmt.Fprint(o.streams.Out, "Stop:  Ctrl+C\n\n")

	if err := pfCmd.Wait(); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(o.streams.ErrOut, "\nStopping port-forward...")
			return nil
		}
		return fmt.Errorf("port-forward: %w", err)
	}

	return nil
}
