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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
	ktesting "k8s.io/kubernetes/test/utils/ktesting"
)

func TestController(t *testing.T) {
	tCtx := ktesting.Init(t)
	tCtx = ktesting.WithCancel(tCtx)
	logger := klog.FromContext(tCtx)

	fakeKubeClient := fake.NewClientset()

	// Create the controller
	c, err := NewController(logger, fakeKubeClient)
	assert.NoError(t, err, "creating dratopology controller")

	// Enqueue a dummy key
	testKey := "test-namespace/test-object"
	c.queue.Add(testKey)

	// Process the work item
	// For a real controller, you might start the Run loop and wait for processing.
	// For this basic test, we directly call processNextWorkItem.
	processed := c.processNextWorkItem(tCtx)
	assert.True(t, processed, "expected work item to be processed")

	// Verify the queue is empty after processing
	assert.Equal(t, 0, c.queue.Len(), "expected queue to be empty after processing")

	// Test shutdown
	c.queue.Add(testKey + "-preshutdown")
	c.queue.ShutDown()
	c.queue.Add(testKey + "-postshutdown")

	processed = c.processNextWorkItem(tCtx)
	assert.True(t, processed, "expected processNextWorkItem to complete work item before shutdown signal")
	processed = c.processNextWorkItem(tCtx)
	assert.False(t, processed, "expected processNextWorkItem to return false for item received after shutdown")
	assert.Equal(t, 0, c.queue.Len(), "expected queue to ignore item after receiving shutdown signal") // Item is not forgotten on shutdown
}

func TestRun(t *testing.T) {
	tCtx := ktesting.Init(t)
	tCtx = ktesting.WithCancel(tCtx)
	logger := klog.FromContext(tCtx)

	fakeKubeClient := fake.NewClientset()

	c, err := NewController(logger, fakeKubeClient)
	assert.NoError(t, err, "creating dratopology controller")

	// Start the controller in a goroutine
	go c.Run(tCtx, 1)

	// Enqueue a key and wait for it to be processed
	testKey := "another-test-key"
	c.queue.Add(testKey)
	assert.Eventually(t, func() bool { return c.queue.Len() == 0 }, 5*time.Second, 100*time.Millisecond, "expected queue to become empty")

	// Stop the controller
	tCtx.Cancel("")

	// Ensure the queue is shut down
	assert.Eventually(t, func() bool { return c.queue.ShuttingDown() }, 5*time.Second, 100*time.Millisecond, "expected queue to be shutting down")
}
