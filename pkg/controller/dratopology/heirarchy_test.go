package dratopology

import (
	"reflect"
	"testing"
)

func TestJSONHeirarchyPlugin_ReadHeirarchy(t *testing.T) {
	plugin := &JSONHeirarchyPlugin{}
	opts := map[string]string{
		"file": "testdata/heirarchy_json.json",
	}

	expectedLevels := []Level{
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

	levels, selectors, count, err := plugin.ReadHeirarchy(opts)
	if err != nil {
		t.Fatalf("ReadHeirarchy() returned an unexpected error: %v", err)
	}

	if !reflect.DeepEqual(levels, expectedLevels) {
		t.Errorf("ReadHeirarchy() levels = %v, want %v", levels, expectedLevels)
	}

	if len(selectors) != len(expectedSelectorLabels) {
		t.Fatalf("ReadHeirarchy() returned %d selectors, want %d", len(selectors), len(expectedSelectorLabels))
	}

	for i, s := range selectors {
		got, _ := s.Requirements()
		want := expectedSelectorLabels[i]
		if !reflect.DeepEqual(got.String(), want) {
			t.Errorf("ReadHeirarchy() selector[%d] = %q, want %q", i, got, want)
		}
	}

	if count != expectedCount {
		t.Errorf("ReadHeirarchy() count = %d, want %d", count, expectedCount)
	}
}
