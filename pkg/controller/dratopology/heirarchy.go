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
	"os"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// HeirarchyPlugin is an interface for an object that can read a specific hierarchy, for example from a JSON file, YAML file, or a Kueue Topology CRD.
type HeirarchyPlugin interface {
	// ReadHeirarchy reads the hierarchy labels from its expected source data.
	ReadHeirarchy(opts map[string]string) ([]Level, []labels.Selector, int, error)
}

// BasicHeirarchy tracks labels for arbitrary named levels and constructs Kubernetes selectors for them.
type BasicHeirarchy struct {
	reader    HeirarchyPlugin
	levels    []Level
	selectors []labels.Selector
	count     int
}

// Level stores data for a topo level's name and associated Kubernetes label string.
type Level struct {
	Name  string
	Label string
}

// NewBasicHeirarchyReader creates a new BasicHeirarchyReader with the configured heirarchy plugin.
func NewBasicHeirarchyReader(plugin HeirarchyPlugin) *BasicHeirarchy {
	return &BasicHeirarchy{
		reader: plugin,
	}
}

// JSON Heirarchy Plugin can read a topo heirarchy from a well-formed JSON file.
type JSONHeirarchyPlugin struct{}

func (h *JSONHeirarchyPlugin) ReadHeirarchy(opts map[string]string) ([]Level, []labels.Selector, int, error) {
	path, ok := opts["file"]
	if !ok {
		return nil, nil, 0, fmt.Errorf("No file provided")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to open JSON file: %w", err)
	}
	defer file.Close()

	var data map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to decode JSON: %w", err)
	}

	var levels []Level
	var selectors []labels.Selector
	count := 0

	// How to walk the heirarchy
	var walk func(map[string]interface{}, []Level) error
	walk = func(currentData map[string]interface{}, currentLevels []Level) error {
		label, ok := currentData["label"].(string)
		if !ok {
			return fmt.Errorf("failed to find label for topology data from JSON: %w", err)
		}
		name, ok := currentData["name"].(string)
		if !ok {
			return fmt.Errorf("failed to find name for topology data from JSON: %w", err)
		}

		currentLevels = append(currentLevels, Level{Name: name, Label: label})

		// Create a proper label selector object for this data.
		// Note: this assumes that Exists is the correct selector for the labels in the heirarchy
		// This may need to be changed or relaxed depending on what other types of label heirarchies we run into
		s := labels.NewSelector()
		r, err := labels.NewRequirement(label, selection.Exists, []string{})
		if err != nil {
			return fmt.Errorf("failed to create label selector requirement: %w", err)
		}
		s = s.Add(*r)
		selectors = append(selectors, s)

		// increase the number of levels we've observed
		count++

		// see if there is further to go in the heirarchy
		if child, ok := currentData["child"]; ok {
			if err := walk(child.(map[string]interface{}), currentLevels); err != nil {
				return fmt.Errorf("failed to walk JSON heirarchy after level %d: %w", count, err)
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
	if err := walk(data, []Level{}); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to walk JSON heirarchy: %w", err)
	}

	return levels, selectors, count, nil

}
