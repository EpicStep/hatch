package knownhosts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Content  string
		Host     string
		Port     int
		Expected string
	}{
		{
			Name:     "StandardPort",
			Content:  "example.com ssh-ed25519 AAAA...\nlocalhost ssh-ed25519 BBBB...\nother.host ssh-rsa CCCC...\n",
			Host:     "localhost",
			Port:     22,
			Expected: "example.com ssh-ed25519 AAAA...\nother.host ssh-rsa CCCC...\n",
		},
		{
			Name:     "NonStandardPort",
			Content:  "[localhost]:2222 ssh-ed25519 AAAA...\nlocalhost ssh-ed25519 BBBB...\n[localhost]:3333 ssh-ed25519 CCCC...\n",
			Host:     "localhost",
			Port:     2222,
			Expected: "localhost ssh-ed25519 BBBB...\n[localhost]:3333 ssh-ed25519 CCCC...\n",
		},
		{
			Name:     "HashedEntriesPreserved",
			Content:  "|1|hash1|hash2 ssh-ed25519 AAAA...\nlocalhost ssh-ed25519 BBBB...\n",
			Host:     "localhost",
			Port:     22,
			Expected: "|1|hash1|hash2 ssh-ed25519 AAAA...\n",
		},
		{
			Name:     "CommentsPreserved",
			Content:  "# comment line\nlocalhost ssh-ed25519 AAAA...\n",
			Host:     "localhost",
			Port:     22,
			Expected: "# comment line\n",
		},
		{
			Name:     "MultiHost",
			Content:  "localhost,127.0.0.1 ssh-ed25519 AAAA...\nother ssh-rsa BBBB...\n",
			Host:     "localhost",
			Port:     22,
			Expected: "other ssh-rsa BBBB...\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "known_hosts")
			require.NoError(t, os.WriteFile(path, []byte(tt.Content), 0o644))

			require.NoError(t, RemoveEntry(path, tt.Host, tt.Port))

			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, tt.Expected, string(data))
		})
	}
}

func TestRemoveEntryFileNotExist(t *testing.T) {
	t.Parallel()

	err := RemoveEntry(filepath.Join(t.TempDir(), "missing"), "localhost", 2222)
	require.NoError(t, err)
}
