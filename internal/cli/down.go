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

func newDownCmd(ho *hatchOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Restore the original workload image",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := ho.config
			ctx := cmd.Context()

			if err := ho.validate(); err != nil {
				return err
			}

			clientset, err := ho.kubeClient()
			if err != nil {
				return err
			}

			namespace := cfg.Namespace

			w, err := workload.New(ctx, clientset, namespace, cfg.Kind, cfg.Workload)
			if err != nil {
				return err
			}

			if w.Annotations()[annotationActive] != "true" {
				fmt.Fprintln(ho.streams.Out, "Dev mode is not active, nothing to do")
				return nil
			}

			originalImage := w.Annotations()[annotationOriginalImage]

			if originalImage == "" {
				return errors.New("annotation " + annotationOriginalImage + " is missing, cannot determine original image")
			}

			activeUser := w.Annotations()[annotationUser]

			if activeUser != "" && activeUser != ho.user {
				fmt.Fprintf(ho.streams.ErrOut, "Warning: dev mode was activated by %s\n", activeUser)
			}

			container, _, err := workload.FindContainer(w, cfg.Container)
			if err != nil {
				return fmt.Errorf("in %s/%s: %w", cfg.Kind, cfg.Workload, err)
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

			fmt.Fprintf(ho.streams.Out, "Restoring %s/%s: image=%s\n", cfg.Kind, cfg.Workload, originalImage)

			if err := w.Patch(ctx, patchBytes, types.StrategicMergePatchType); err != nil {
				return fmt.Errorf("restoring workload: %w", err)
			}

			fmt.Fprintf(ho.streams.Out, "Restored %s/%s to %s\n", cfg.Kind, cfg.Workload, originalImage)
			return nil
		},
	}
}
