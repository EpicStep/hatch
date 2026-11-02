package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/EpicStep/hatch/internal/workload"
)

func newDownCmd(opts *generalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Restore the original workload image",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDown(cmd, opts)
		},
	}
}

func runDown(cmd *cobra.Command, opts *generalOptions) error {
	ctx := cmd.Context()

	client, err := opts.kubeClient()
	if err != nil {
		return err
	}

	w, err := workload.New(ctx, client, opts.namespace, opts.kind, opts.workload)
	if err != nil {
		return err
	}

	if w.Annotations()[annotationActive] != "true" {
		fmt.Println("Dev mode is not active, nothing to do")
		return nil
	}

	raw := w.Annotations()[annotationOriginal]
	if raw == "" {
		return errors.New("annotation " + annotationOriginal + " is missing, cannot restore the original container (dev mode started by hatch 0.0.8 or older must be stopped with that version)")
	}

	var original originalContainer
	if err = json.Unmarshal([]byte(raw), &original); err != nil {
		return fmt.Errorf("parsing annotation %s: %w", annotationOriginal, err)
	}

	activeUser := w.Annotations()[annotationUser]
	if activeUser != "" && activeUser != opts.user {
		fmt.Printf("Warning: dev mode was activated by %s\n", activeUser)
	}

	container, err := workload.FindContainer(w, opts.container)
	if err != nil {
		return fmt.Errorf("in %s/%s: %w", opts.kind, opts.workload, err)
	}

	before := w.DeepCopy()

	container.Image = original.Image
	container.Command = original.Command
	container.Args = original.Args
	container.SecurityContext = original.SecurityContext
	container.Env = slices.DeleteFunc(container.Env, func(e corev1.EnvVar) bool {
		return e.Name == "AUTHORIZED_KEYS"
	})

	ann := w.Annotations()
	delete(ann, annotationActive)
	delete(ann, annotationOriginal)
	delete(ann, annotationUser)
	delete(ann, annotationNode)
	delete(ann, annotationPod)

	patchBytes, err := workload.MergeFrom(before, w.Object())
	if err != nil {
		return fmt.Errorf("building patch: %w", err)
	}

	fmt.Printf("Restoring %s/%s: image=%s\n", opts.kind, opts.workload, original.Image)

	if err = w.Patch(ctx, patchBytes, types.StrategicMergePatchType); err != nil {
		return fmt.Errorf("restoring workload: %w", err)
	}

	fmt.Printf("Restored %s/%s to %s\n", opts.kind, opts.workload, original.Image)

	return nil
}
