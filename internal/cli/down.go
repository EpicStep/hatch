package cli

import (
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

			originalImage := w.Annotations()[annotationOriginalImage]
			if originalImage == "" {
				return errors.New("annotation " + annotationOriginalImage + " is missing, cannot determine original image")
			}

			activeUser := w.Annotations()[annotationUser]
			if activeUser != "" && activeUser != opts.user {
				fmt.Printf("Warning: dev mode was activated by %s\n", activeUser)
			}

			container, err := workload.FindContainer(w, opts.container)
			if err != nil {
				return fmt.Errorf("in %s/%s: %w", opts.kind, opts.workload, err)
			}

			// Single atomic patch: restore image, remove env, delete annotations.
			before := w.DeepCopy()

			container.Image = originalImage
			container.Env = slices.DeleteFunc(container.Env, func(e corev1.EnvVar) bool {
				return e.Name == "AUTHORIZED_KEYS"
			})

			ann := w.Annotations()
			delete(ann, annotationActive)
			delete(ann, annotationOriginalImage)
			delete(ann, annotationUser)
			delete(ann, annotationNode)
			delete(ann, annotationPod)

			patchBytes, err := workload.MergeFrom(before, w.Object())
			if err != nil {
				return fmt.Errorf("building patch: %w", err)
			}

			fmt.Printf("Restoring %s/%s: image=%s\n", opts.kind, opts.workload, originalImage)

			if err = w.Patch(ctx, patchBytes, types.StrategicMergePatchType); err != nil {
				return fmt.Errorf("restoring workload: %w", err)
			}

			fmt.Printf("Restored %s/%s to %s\n", opts.kind, opts.workload, originalImage)
			return nil
		},
	}
}
