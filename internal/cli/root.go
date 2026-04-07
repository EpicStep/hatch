package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/EpicStep/hatch/internal/config"
)

type generalOptions struct {
	k8sConfig *genericclioptions.ConfigFlags

	configPath string
	namespace  string
	kind       string
	workload   string
	container  string
	image      string
	user       string
}

func (opts *generalOptions) validate() error {
	if opts.workload == "" {
		return errors.New("workload name is required (set in .hatch.yaml or use --workload)")
	}

	if opts.container == "" {
		return errors.New("container name is required (set in .hatch.yaml or use --container)")
	}

	return nil
}

func (opts *generalOptions) setDefaults() {
	if opts.kind == "" {
		opts.kind = "daemonset"
	}

	if opts.image == "" {
		opts.image = "ghcr.io/epicstep/hatch:v0.0.1"
	}

	if opts.user == "" {
		opts.user = os.Getenv("USER")
	}
}

func (opts *generalOptions) kubeClient() (kubernetes.Interface, error) {
	restConfig, err := opts.k8sConfig.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("opts.k8sConfig.ToRESTConfig: %w", err)
	}

	return kubernetes.NewForConfig(restConfig)
}

// NewRootCmd creates the top-level hatch cobra command.
func NewRootCmd(version string) *cobra.Command {
	opts := &generalOptions{
		k8sConfig: genericclioptions.NewConfigFlags(true),
	}

	cmd := &cobra.Command{
		Use:           "hatch",
		Short:         "Dev SSH into K8s pods via image swap",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			configFromFile, err := config.Load(opts.configPath)
			if err != nil {
				return fmt.Errorf("config.Load: %w", err)
			}

			if cmd.Flags().Changed("namespace") {
				opts.namespace = *opts.k8sConfig.Namespace
			} else if configFromFile.Namespace != "" {
				opts.namespace = configFromFile.Namespace
			} else {
				ns, _, err := opts.k8sConfig.ToRawKubeConfigLoader().Namespace()
				if err != nil {
					return fmt.Errorf("resolving namespace from kubeconfig: %w", err)
				}

				opts.namespace = ns
			}

			if !cmd.Flags().Changed("kind") {
				opts.kind = configFromFile.Kind
			}

			if !cmd.Flags().Changed("workload") {
				opts.workload = configFromFile.Workload
			}

			if !cmd.Flags().Changed("container") {
				opts.container = configFromFile.Container
			}

			if !cmd.Flags().Changed("image") {
				opts.image = configFromFile.Image
			}

			opts.setDefaults()

			if err = opts.validate(); err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "path to .hatch.yaml")
	cmd.PersistentFlags().StringVar(&opts.kind, "kind", "", "workload kind: daemonset, deployment, statefulset")
	cmd.PersistentFlags().StringVar(&opts.workload, "workload", "", "workload name")
	cmd.PersistentFlags().StringVar(&opts.container, "container", "", "container name in the pod spec")
	cmd.PersistentFlags().StringVar(&opts.image, "image", "", "dev image reference")
	opts.k8sConfig.AddFlags(cmd.PersistentFlags())

	cmd.PersistentFlags().StringVar(&opts.user, "dev-user", "", "user identifier for multi-user environments (default: $USER)")

	cmd.AddCommand(newUpCmd(opts))
	cmd.AddCommand(newDownCmd(opts))
	cmd.AddCommand(newStatusCmd(opts))

	return cmd
}
