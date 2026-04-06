package cli

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/EpicStep/hatch/internal/config"
)

type hatchOptions struct {
	configFlags *genericclioptions.ConfigFlags
	config      *config.Config
	streams     genericclioptions.IOStreams

	configPath string
	kind       string
	workload   string
	container  string
	image      string
	user       string
}

// NewRootCmd creates the top-level hatch cobra command.
func NewRootCmd(version string, streams genericclioptions.IOStreams) *cobra.Command {
	o := &hatchOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		streams:     streams,
	}

	cmd := &cobra.Command{
		Use:           "hatch",
		Short:         "Dev SSH into K8s pods via image swap",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(o.configPath)
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("kind") {
				cfg.Kind = o.kind
			}
			if cmd.Flags().Changed("workload") {
				cfg.Workload = o.workload
			}
			if cmd.Flags().Changed("container") {
				cfg.Container = o.container
			}
			if cmd.Flags().Changed("image") {
				cfg.Image = o.image
			}
			if cmd.Flags().Changed("namespace") {
				cfg.Namespace = *o.configFlags.Namespace
			}

			cfg.ApplyDefaults()

			if o.user == "" {
				o.user = os.Getenv("USER")
			}

			// Resolve namespace from kubeconfig context if still at default.
			if !cmd.Flags().Changed("namespace") && cfg.Namespace == "default" {
				if ns, _, err := o.configFlags.ToRawKubeConfigLoader().Namespace(); err == nil && ns != "" {
					cfg.Namespace = ns
				}
			}

			o.config = cfg

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&o.configPath, "config", "", "path to .hatch.yaml")
	cmd.PersistentFlags().StringVar(&o.kind, "kind", "", "workload kind: daemonset, deployment, statefulset")
	cmd.PersistentFlags().StringVar(&o.workload, "workload", "", "workload name")
	cmd.PersistentFlags().StringVar(&o.container, "container", "", "container name in the pod spec")
	cmd.PersistentFlags().StringVar(&o.image, "image", "", "dev image reference")
	o.configFlags.AddFlags(cmd.PersistentFlags())

	cmd.PersistentFlags().StringVar(&o.user, "dev-user", "", "user identifier for multi-user environments (default: $USER)")

	cmd.AddCommand(newUpCmd(o))
	cmd.AddCommand(newDownCmd(o))
	cmd.AddCommand(newStatusCmd(o))

	return cmd
}
