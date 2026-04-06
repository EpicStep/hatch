package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/EpicStep/hatch/internal/workload"
)

func newStatusCmd(ho *hatchOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current dev environment status",
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

			if _, _, err := workload.FindContainer(w, cfg.Container); err != nil {
				return fmt.Errorf("in %s/%s: %w", cfg.Kind, cfg.Workload, err)
			}

			var containerImage string
			for _, c := range w.PodSpec().Containers {
				if c.Name == cfg.Container {
					containerImage = c.Image
					break
				}
			}

			out := ho.streams.Out
			fmt.Fprintf(out, "Workload:   %s/%s\n", cfg.Kind, cfg.Workload)
			fmt.Fprintf(out, "Namespace:  %s\n", namespace)
			fmt.Fprintf(out, "Container:  %s\n", cfg.Container)
			fmt.Fprintf(out, "Image:      %s\n", containerImage)

			ann := w.Annotations()
			if ann[annotationActive] == "true" {
				devInfo := fmt.Sprintf("ACTIVE (by %s)", ann[annotationUser])
				if n := ann[annotationNode]; n != "" {
					devInfo += fmt.Sprintf(" on node %s", n)
				}
				if p := ann[annotationPod]; p != "" {
					devInfo += fmt.Sprintf(" pod %s", p)
				}
				fmt.Fprintf(out, "Dev Mode:   %s\n", devInfo)
				fmt.Fprintf(out, "Original:   %s\n", ann[annotationOriginalImage])
			} else {
				fmt.Fprintln(out, "Dev Mode:   INACTIVE")
			}
			fmt.Fprintln(out)

			pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labels.Set(w.Selector()).String(),
			})
			if err != nil {
				return err
			}

			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NODE\tPOD\tSTATUS\tIP")
			for _, pod := range pods.Items {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", pod.Spec.NodeName, pod.Name, string(pod.Status.Phase), pod.Status.PodIP)
			}
			return tw.Flush()
		},
	}
}
