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

	"k8s.io/apimachinery/pkg/util/diff"
)

func TestJSONHierarchyPlugin_ReadHierarchy(t *testing.T) {
	plugin := &JSONHierarchyPlugin{path: "testdata/hierarchy_json.json"}

	expectedLevels := []FlatLevel{
		{Name: "block", Label: "topo.example.com/block"},
		{Name: "subblock", Label: "topo.example.com/subblock"},
		{Name: "host", Label: "topo.example.com/host"},
	}
	expectedSelectorLabels := []string{
		"topo.example.com/block",
		"topo.example.com/subblock",
		"topo.example.com/host",
	}
	expectedCount := 3

	levels, selectors, count, err := plugin.ReadHierarchy()
	if err != nil {
		t.Fatalf("ReadHierarchy() returned an unexpected error: %v", err)
	}

	if d := diff.Diff(levels, expectedLevels); d != "" {
		t.Errorf("ReadHierarchy() levels = %v", d)
	}

	if len(selectors) != len(expectedSelectorLabels) {
		t.Fatalf("ReadHierarchy() returned %d selectors, want %d", len(selectors), len(expectedSelectorLabels))
	}

	for i, s := range selectors {
		got, _ := s.Requirements()
		want := expectedSelectorLabels[i]
		if d := diff.Diff(got.String(), want); d != "" {
			t.Errorf("ReadHierarchy() selector[%d] = %s", i, d)
		}
	}

	if count != expectedCount {
		t.Errorf("ReadHierarchy() count = %d, want %d", count, expectedCount)
	}
}
