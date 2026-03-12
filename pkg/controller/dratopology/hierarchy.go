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
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// HierarchyPlugin is an interface for an object that can read a specific hierarchy, for example from a JSON file, YAML file, or a Kueue Topology CRD.
type HierarchyPlugin interface {
	// ReadHierarchy reads the hierarchy labels from its expected source data,
	// returning a flattened list of all the topo levels, an ordered list of the
	// Kubernetes Selector objects that represent the same indexed topo levels,
	// and the number of levels obeserved (used for testing).
	ReadHierarchy() ([]FlatLevel, []labels.Selector, int, error)
}

// FlatHierarchy can read and store a flattened list of topo levels, including constructed Kubernetes selectors for them.
type FlatHierarchy struct {
	reader    HierarchyPlugin
	levels    []FlatLevel
	selectors []labels.Selector
	count     int
}

// Data stores the unmarshalled data from a topology data source.
type Data struct {
	Version int
	Root    Level
}

// Level stores unmarshalled data for a root branch from a topology data source.
type Level struct {
	Name     string
	Label    string
	Children []*Level
}

// FlatLevel stores data for an individual topo level without its children
type FlatLevel struct {
	Name  string
	Label string
}

// NewBasicHierarchyReader creates a new BasicHierarchyReader with the configured hierarchy plugin.
func NewFlatHierarchyReader(plugin HierarchyPlugin) *FlatHierarchy {
	return &FlatHierarchy{
		reader: plugin,
	}
}

// JSON Hierarchy Plugin can read a topo hierarchy from a well-formed JSON file.
type JSONHierarchyPlugin struct {
	path string
	fs   fs.FS
}

func (h *JSONHierarchyPlugin) ReadHierarchy() ([]FlatLevel, []labels.Selector, int, error) {
	if h.path == "" {
		return nil, nil, 0, fmt.Errorf("No file provided")
	}
	if h.fs == nil {
		h.fs = os.DirFS("/")
	}

	path := h.path
	// remove leading slash as it will be added by the fsys
	if strings.HasPrefix(h.path, "/") {
		path = h.path[1:]
	}

	fdata, err := fs.ReadFile(h.fs, path)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to open file. path: %s, fs: %s, error: %w", path, h.fs, err)
	}

	var data Data
	if err = json.Unmarshal(fdata, &data); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to decode JSON: %w", err)
	}

	var levels []FlatLevel
	var selectors []labels.Selector
	count := 0

	// Flatten the hierarchy into individual name/label pairs

	// How to walk the hierarchy
	var walk func(Level, []FlatLevel) error
	walk = func(l Level, currentLevels []FlatLevel) error {
		if l.Label == "" {
			return fmt.Errorf("failed to find label for topology data from JSON: %w", err)
		}
		if l.Name == "" {
			return fmt.Errorf("failed to find name for topology data from JSON: %w", err)
		}

		currentLevels = append(currentLevels, FlatLevel{Name: l.Name, Label: l.Label})

		s, err := constructLabel(l)
		if err != nil {
			return fmt.Errorf("failed to find construct Kubernetes Selector object for label %v: %w", l, err)
		}
		selectors = append(selectors, s)

		// increase the number of levels we've observed
		count++

		// see if there is further to go in the hierarchy
		if len(l.Children) > 0 {
			for _, child := range l.Children {
				if err := walk(*child, currentLevels); err != nil {
					return fmt.Errorf("failed to walk JSON hierarchy after level %d: %w", count, err)
				}
			}
		} else {
			// If no children, this is a leaf node, so add its levels
			if len(currentLevels) > len(levels) {
				levels = currentLevels
			}
		}
		return nil
	}

	// Start walking from the root of the JSON data
	if err := walk(data.Root, []FlatLevel{}); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to walk JSON hierarchy: %w", err)
	}

	return levels, selectors, count, nil

}

// constructLabel creates a labels.Selector object for the described label
func constructLabel(l Level) (labels.Selector, error) {
	// Note: this assumes that Exists is the correct selector for the labels in the hierarchy
	// This may need to be changed or relaxed depending on what other types of label heirarchies we run into
	s := labels.NewSelector()
	r, err := labels.NewRequirement(l.Label, selection.Exists, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to create label selector requirement: %w", err)
	}
	s = s.Add(*r)
	return s, nil
}
