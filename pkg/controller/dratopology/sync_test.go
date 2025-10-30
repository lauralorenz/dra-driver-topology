/*
Copyright 2025 The Kubernetes Authors.

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

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/test/utils/ktesting"
)

func TestEnqueueNode(t *testing.T) {

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(node)
	if err != nil {
		t.Fatalf("Unexpected error getting key for node: %v", err)
	}

	testCases := []struct {
		name        string
		obj         interface{}
		expectedKey string
	}{
		{
			name:        "valid node",
			obj:         node,
			expectedKey: key,
		},
		{
			name: "deleted final state unknown",
			obj: cache.DeletedFinalStateUnknown{
				Key: key,
				Obj: node,
			},
			expectedKey: key,
		},
		{
			name:        "invalid object",
			obj:         "not-a-node",
			expectedKey: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

			c := &Controller{
				nodeLister: &testNodeLister{indexer: indexer},
				queue: workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[string](),
					workqueue.TypedRateLimitingQueueConfig[string]{Name: tc.name},
				),
			}

			c.enqueueNode(tc.obj)

			if tc.expectedKey == "" {
				if c.queue.Len() != 0 {
					t.Errorf("Expected queue to be empty, but got %d items", c.queue.Len())
				}
			} else {
				if c.queue.Len() != 1 {
					t.Errorf("Expected queue to have 1 item, but got %d", c.queue.Len())
				}
				item, _ := c.queue.Get()
				if item != tc.expectedKey {
					t.Errorf("Expected key %q, but got %q", tc.expectedKey, item)
				}
			}
		})
	}
}

func TestSyncNode(t *testing.T) {
	// TODO: change test cases to take nodes with labels in them

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if err := indexer.Add(node); err != nil {
		t.Fatalf("Unexpected error adding node to indexer: %v", err)
	}

	testCases := []struct {
		name      string
		key       string
		expectErr bool
	}{
		{
			name:      "node exists",
			key:       "test-node",
			expectErr: false,
		},
		{
			name:      "node does not exist",
			key:       "non-existent-node",
			expectErr: false, // Not found is not an error
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tCtx := ktesting.Init(t)
			tCtx = ktesting.WithCancel(tCtx)
			c := &Controller{
				nodeLister: &testNodeLister{indexer: indexer},
			}

			err := c.syncNode(tCtx, tc.key)

			if tc.expectErr && err == nil {
				t.Error("Expected an error, but got none")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// fake node lister for tests
type testNodeLister struct {
	indexer cache.Indexer
}

func (l *testNodeLister) Get(name string) (*v1.Node, error) {
	obj, exists, err := l.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierrors.NewNotFound(v1.Resource("node"), name)
	}
	return obj.(*v1.Node), nil
}

func (l *testNodeLister) List(labels labels.Selector) ([]*v1.Node, error) {
	// This is not used in the tested functions
	return nil, nil
}
