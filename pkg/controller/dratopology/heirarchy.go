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
	ReadHeirarchy(opts map[string]string) ([]string, []labels.Selector, int, error)
}

// BasicHeirarchy tracks label selectors for block, subblock, and host.
type BasicHeirarchy struct {
	reader    HeirarchyPlugin
	levels    []string
	selectors []labels.Selector
	count     int
}

// NewBasicHeirarchyReader creates a new BasicHeirarchyReader with the configured heirarchy plugin.
func NewBasicHeirarchyReader(plugin HeirarchyPlugin) *BasicHeirarchy {
	return &BasicHeirarchy{
		reader: plugin,
	}
}

type JSONHeirarchyPlugin struct{}

func (h *JSONHeirarchyPlugin) ReadHeirarchy(opts map[string]string) ([]string, []labels.Selector, int, error) {
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

	var levels []string
	var selectors []labels.Selector
	count := 0

	// How to walk the heirarchy
	var walk func(map[string]interface{}, []string) error
	walk = func(currentData map[string]interface{}, currentLevels []string) error {
		label, ok := currentData["label"].(string)
		if !ok {
			return fmt.Errorf("failed to find label for topology data from JSON: %w", err)
		}
		currentLevels = append(currentLevels, label)

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
	if err := walk(data, []string{}); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to walk JSON heirarchy: %w", err)
	}

	return levels, selectors, count, nil

}
