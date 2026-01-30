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

package dratopology

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerVersion = "v0.0.1"
)

// ControllerOptions contains options for the DRATopologyController
type ControllerOptions struct {
	// JSONFilePath is the file path to a JSON file containing the topology level data
	JSONFilePath string
}

// Controller is the dratopology controller.
type Controller struct {
	kubeClient clientset.Interface
	queue      workqueue.TypedRateLimitingInterface[string]
	logger     klog.Logger
	options    ControllerOptions
}

// NewController creates a new dratopology controller.
func NewController(logger klog.Logger, kubeClient clientset.Interface, options ControllerOptions) (*Controller, error) {
	c := &Controller{
		kubeClient: kubeClient,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "dratopology"},
		),
		logger:  logger,
		options: options,
	}

	return c, nil
}

// Run starts the dratopology controller.
func (c *Controller) Run(parent context.Context, workers int) {

	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)

	c.logger.Info("Starting dratopology controller")

	// Create device classes for each known topology type first
	c.createTopologyDeviceClasses(ctx)

	// then start workers to process new node events to add devices as they are seen
	for range workers {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
	stop()

	c.logger.Info("Shutting down gracefully...")
	c.ShutDown()
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(ctx, key)
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	runtime.HandleError(fmt.Errorf("%v failed with: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

// syncHandler is invoked for each work item.
func (c *Controller) syncHandler(ctx context.Context, key string) error {
	c.logger.V(4).Info("Processing key", "key", key)
	// In a real controller, this is where you would fetch the object
	// identified by 'key' and reconcile its state.
	return nil
}

func (c *Controller) ShutDown() {
	c.logger.Info("Shutting down dratopology controller")
	runtime.HandleCrash()
	c.queue.ShutDown()
}

func (c *Controller) createTopologyDeviceClasses(ctx context.Context) {
	c.logger.Info(fmt.Sprintf("creating topology device classes with options %v", c.options))

	var plugin HierarchyPlugin
	var opts map[string]string

	switch {
	case c.options.JSONFilePath != "":
		plugin = &JSONHierarchyPlugin{}
		opts = map[string]string{"file": c.options.JSONFilePath}
	default:
		c.logger.Error(nil, "no topology source specified. You must provide a well-formed JSON to --topology-config-json.")
		return
	}

	levels, selectors, count, err := plugin.ReadHierarchy(opts)
	if err != nil {
		c.logger.Error(err, "failed to read hierarchy")
		return
	}

	hierarchy := &BasicHierarchy{
		levels:    levels,
		selectors: selectors,
		count:     count,
	}

	if err := SyncDeviceClasses(ctx, c.kubeClient, hierarchy); err != nil {
		c.logger.Error(err, "failed to sync device classes")
	}
}
