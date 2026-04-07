package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/EpicStep/hatch/internal/workload"
)

func newStatusCmd(opts *generalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current dev environment status",
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

			if _, err = workload.FindContainer(w, opts.container); err != nil {
				return fmt.Errorf("in %s/%s: %w", opts.kind, opts.workload, err)
			}

			var containerImage string
			for _, c := range w.PodSpec().Containers {
				if c.Name != opts.container {
					continue
				}

				containerImage = c.Image
				break
			}

			fmt.Printf("Workload:   %s/%s\n", opts.kind, opts.workload)
			fmt.Printf("Namespace:  %s\n", opts.namespace)
			fmt.Printf("Container:  %s\n", opts.container)
			fmt.Printf("Image:      %s\n", containerImage)

			ann := w.Annotations()
			if ann[annotationActive] == "true" {
				devInfo := fmt.Sprintf("ACTIVE (by %s)", ann[annotationUser])
				if n := ann[annotationNode]; n != "" {
					devInfo += fmt.Sprintf(" on node %s", n)
				}

				if p := ann[annotationPod]; p != "" {
					devInfo += fmt.Sprintf(" pod %s", p)
				}

				fmt.Printf("Dev Mode:   %s\n", devInfo)
				fmt.Printf("Original:   %s\n", ann[annotationOriginalImage])
			} else {
				fmt.Print("Dev Mode:   INACTIVE")
			}

			fmt.Println()

			pods, err := client.CoreV1().Pods(opts.namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labels.Set(w.Selector()).String(),
			})
			if err != nil {
				return err
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NODE\tPOD\tSTATUS\tIP")
			for _, pod := range pods.Items {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", pod.Spec.NodeName, pod.Name, string(pod.Status.Phase), pod.Status.PodIP)
			}

			return tw.Flush()
		},
	}
}
