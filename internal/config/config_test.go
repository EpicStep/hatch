package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Content  string
		Expected *Config
		WantErr  bool
	}{
		{
			Name:     "FileNotExist",
			Content:  "",
			Expected: &Config{},
		},
		{
			Name:    "ValidFile",
			Content: "namespace: test-ns\nkind: deployment\nworkload: my-app\ncontainer: app\nimage: myimage:dev\n",
			Expected: &Config{
				Namespace: "test-ns",
				Kind:      "deployment",
				Workload:  "my-app",
				Container: "app",
				Image:     "myimage:dev",
			},
		},
		{
			Name:    "InvalidYAML",
			Content: ":\n\t:bad",
			WantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, ".hatch.yaml")

			if tt.Content == "" && !tt.WantErr {
				// don't create the file — test missing file case
				path = filepath.Join(dir, "missing.yaml")
			} else {
				require.NoError(t, os.WriteFile(path, []byte(tt.Content), 0o644))
			}

			cfg, err := Load(path)
			if tt.WantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.Expected, cfg)
		})
	}
}
