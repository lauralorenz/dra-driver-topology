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

package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2/ktesting"
)

// TestNewControllerCommand simply ensures that the NewControllerCommand function builds a cobra command that
// has the expected name and has flags registered.
func TestNewControllerCommand(t *testing.T) {
	cmd := NewControllerCommand()
	assert.Equal(t, "dratopology-controller", cmd.Use)
	assert.True(t, cmd.HasFlags())
}

// TestRun_InvalidClusterConfig ensures that the Run function returns an error when an invalid config is provided.
func TestRun_InvalidClusterConfig(t *testing.T) {
	opts := &Options{
		ConfigOverrides:        clientcmd.ConfigOverrides{CurrentContext: "non-existent-context"},
		TopologyConfigJSONPath: "wont-get-here",
	}
	err := Run(context.Background(), opts)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "could not build cluster config from provided options")
}

func TestRun_NotEnoughConfig(t *testing.T) {
	p := "../../pkg/controller/dratopology/testdata/hierarchy_json.json"
	absPath, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("Failed to get absolute path for %s: %v", p, err)
	}

	testcases := []struct {
		name                   string
		args                   []string
		expectedErr            bool
		expectedErrExactString string
	}{
		{
			name:                   "needs cluster",
			args:                   []string{"--topology-config-json", absPath},
			expectedErr:            true,
			expectedErrExactString: "could not build cluster config from provided options: invalid configuration: no configuration has been provided, try setting KUBERNETES_MASTER environment variable",
		},
		{
			name:                   "needs --topology-config-json",
			args:                   []string{"--kube-server", "randomurl"},
			expectedErr:            true,
			expectedErrExactString: "you must provide --topology-config-json",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, tctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithTimeout(tctx, time.Duration(3*time.Second))
			defer cancel()
			cmd := NewControllerCommand()
			cmd.SetArgs(tc.args)
			err := cmd.ExecuteContext(ctx)
			if !tc.expectedErr {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if d := diff.Diff(err.Error(), tc.expectedErrExactString); d != "" {
					t.Errorf("Unexpected error: %v", d)
				}
			}
		})
	}
}
