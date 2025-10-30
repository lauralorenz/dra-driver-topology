/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUTHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dratopology

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// SetupNodeInformer sets up an informer for Nodes.
// This function can be called when creating a new controller.
func (c *Controller) SetupNodeInformer(nodeInformer v1informers.NodeInformer) {
	nodeInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.enqueueNode,
			UpdateFunc: func(oldObj, newObj interface{}) { c.enqueueNode(newObj) },
			DeleteFunc: c.enqueueNode,
		},
	)
}

func (c *Controller) enqueueNode(obj interface{}) {
	node, ok := obj.(*v1.Node)
	// TODO: is this correct?
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		node, ok = tombstone.Obj.(*v1.Node)
		if !ok {
			runtime.HandleError(fmt.Errorf("tombstone contained object that is not a Node %#v", obj))
			return
		}
	}
	key, err := cache.MetaNamespaceKeyFunc(node)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// syncNode processes a node from the work queue.
func (c *Controller) syncNode(ctx context.Context, key string) error {
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "node", key)

	logger.V(4).Info("processing node")

	node, err := c.nodeLister.Get(key)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(4).Info("node has been deleted")
			// Node is gone, TODO: check if we still need its device
			return nil
		}
		return err
	}

	// Here would be the logic to handle the node update.
	// For now, we just log the node's name and some topology information.
	// TODO: create device in resourceslice
	logger.V(4).Info("syncing node", "nodeName", node.Name)
	for key, value := range node.Labels {
		// This is just an example of what could be done.
		if key == "kubernetes.io/hostname" {
			logger.V(4).Info("node has hostname", "nodeName", node.Name, "hostname", value)
		}
	}

	return nil
}
