/*
Copyright 2024 The Kubernetes Authors.

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
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1informers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// ControllerOptions contains options for the DRATopologyController
type ControllerOptions struct {
	// JSONFilePath is the file path to a JSON file containing the topology level data
	JSONFilePath string
}

// Controller is the dratopology controller.
type Controller struct {
	kubeClient   clientset.Interface
	nodeInformer v1informers.NodeInformer
	nodeLister   v1listers.NodeLister
	queue        workqueue.TypedRateLimitingInterface[string]
	logger       klog.Logger
	options      ControllerOptions
}

// NewController creates a new dratopology controller.
func NewController(logger klog.Logger, kubeClient clientset.Interface, nodeInformer v1informers.NodeInformer, options ControllerOptions) (*Controller, error) {
	c := &Controller{
		kubeClient: kubeClient,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "dratopology"},
		),
		nodeInformer: nodeInformer,
		nodeLister:   nodeInformer.Lister(),
		logger:       logger,
		options:      options,
	}

	c.SetupNodeInformer(nodeInformer)

	return c, nil
}

// Run starts the dratopology controller.
func (c *Controller) Run(parent context.Context, workers int) {
	ctx, cancel := context.WithCancel(parent)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func(ctx context.Context, c *Controller) {
		<-shutdown
		c.logger.Info("Shutting down gracefully...")
		c.ShutDown(ctx)
		cancel()
	}(ctx, c)

	c.logger.Info("Starting dratopology controller")

	// Create device classes for each known topology type first
	c.createTopologyDeviceClasses()

	// then start workers to process new node events to add devices as they are seen
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
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
	c.syncNode(ctx, key)
	return nil
}

func (c *Controller) ShutDown(ctx context.Context) {
	c.logger.Info("Shutting down dratopology controller")
	runtime.HandleCrash()
	c.queue.ShutDown()
}

func (c *Controller) createTopologyDeviceClasses() {
	c.logger.Info(fmt.Sprintf("creating topology device classes with options %v", c.options))
	// TODO: interpret the heirarchy data into deviceclass objects
}
