/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/dra-driver-topology/pkg/controller/dratopology"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/term"
	"k8s.io/controller-manager/pkg/clientbuilder"
	"k8s.io/klog/v2"
)

const (
	dratopologyControllerName = "dratopology-controller"
)

// Options contains the options for running the controller.
type Options struct {
	Logs            *logs.Options
	ConfigOverrides clientcmd.ConfigOverrides
}

// ControllerContext defines the context object for the controller.
// It is a simplified version of the ControllerContext used by kube-controller-manager for its managed controllers.
type ControllerContext struct {
	// ClientBuilder will provide a client for this controller to use
	ClientBuilder clientbuilder.ControllerClientBuilder
	// ResyncPeriod function generates a wait for the controller between apiserver requests;
	// In kube-controller-manager this is a function in order to make it variable per controller managed.
	ResyncPeriod func() time.Duration
}

// ResyncPeriod generates a wait for the controller between apiserver requests.
func ResyncPeriod() time.Duration {
	// When running this controller alone, don't bother to jitter.
	return 10 * time.Second
}

func (o *Options) Flags() cliflag.NamedFlagSets {
	var nfs cliflag.NamedFlagSets

	logsapi.AddFlags(o.Logs, nfs.FlagSet("logs"))

	overrideFlags := clientcmd.RecommendedConfigOverrideFlags("kube-")
	clientcmd.BindOverrideFlags(&o.ConfigOverrides, nfs.FlagSet("kubeconfig"), overrideFlags)

	return nfs
}

func NewControllerCommand() *cobra.Command {
	opts := &Options{
		Logs: logs.NewOptions(),
	}

	cmd := &cobra.Command{
		Use:  dratopologyControllerName,
		Long: `The DRA Topology Controller is a controller that manages ResourceSlices that represent Kubernetes node topology.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// Activate logging as soon as possible, after that
			// show flags with the final logging configuration.
			if err := logsapi.ValidateAndApply(opts.Logs, nil); err != nil {
				return err
			}
			cliflag.PrintFlags(cmd.Flags())

			ctx := cmd.Context()
			return Run(ctx, opts)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	fs := cmd.Flags()
	namedFlagSets := opts.Flags()

	fs.AddFlagSet(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name(), logs.SkipLoggingConfigurationFlags())

	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cliflag.SetUsageAndHelpFunc(cmd, namedFlagSets, cols)

	return cmd
}

func Run(ctx context.Context, opts *Options) error {
	logger := klog.FromContext(ctx)
	version := "v0.0.1"
	logger.Info(fmt.Sprintf("Starting %s, version %s", dratopologyControllerName, version))

	// Get control plane config
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &opts.ConfigOverrides)
	cfg, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("could not build cluster config from provided options: %w", err)
	}
	// Build context for controller
	controllerContext := ControllerContext{
		ClientBuilder: clientbuilder.SimpleControllerClientBuilder{
			ClientConfig: cfg,
		},
		ResyncPeriod: ResyncPeriod,
	}

	// Construct controller
	controller, err := dratopology.NewController(logger, controllerContext.ClientBuilder.ClientOrDie(dratopologyControllerName))
	if err != nil {
		return fmt.Errorf("could not construct controller: %w", err)
	}

	// Run directly.
	run(ctx, *controller, opts)
	return nil

	// TODO: Run with leader election
}

func run(ctx context.Context, controller dratopology.Controller, opts *Options) {
	// run the controller
	controller.Run(ctx, 1)
}
