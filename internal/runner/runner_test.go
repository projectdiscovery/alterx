package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/projectdiscovery/goflags"
	"github.com/stretchr/testify/require"
)

func TestParseFlagsInput(t *testing.T) {
	dir := t.TempDir()
	listPath := filepath.Join(dir, "domains.txt")
	require.NoError(t, os.WriteFile(listPath, []byte("file.example.com\nsecond.example.com\n"), 0600))
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("list:\n  - config.example.com\n"), 0600))

	tests := []struct {
		name     string
		args     []string
		stdin    string
		keepOpen bool
		want     []string
		wantErr  string
	}{
		{
			name: "short list with pipe data", args: []string{"-l", "cli.example.com,second.example.com"},
			stdin: "pipe.example.com\n", want: []string{"cli.example.com", "second.example.com"},
		},
		{
			name: "long list with pipe data", args: []string{"-list", "cli.example.com"},
			stdin: "pipe.example.com\n", want: []string{"cli.example.com"},
		},
		{
			name: "file with pipe data", args: []string{"-l", listPath},
			stdin: "pipe.example.com\n", want: []string{"file.example.com", "second.example.com"},
		},
		{
			name: "config with pipe data", args: []string{"-config", configPath},
			stdin: "pipe.example.com\n", want: []string{"config.example.com"},
		},
		{
			name: "short list with idle pipe", args: []string{"-l", "cli.example.com"},
			keepOpen: true, want: []string{"cli.example.com"},
		},
		{
			name: "long list with idle pipe", args: []string{"-list", "cli.example.com"},
			keepOpen: true, want: []string{"cli.example.com"},
		},
		{
			name: "file with idle pipe", args: []string{"-list", listPath},
			keepOpen: true, want: []string{"file.example.com", "second.example.com"},
		},
		{
			name: "config with idle pipe", args: []string{"-config", configPath},
			keepOpen: true, want: []string{"config.example.com"},
		},
		{
			name: "list with empty pipe", args: []string{"-l", "cli.example.com"},
			want: []string{"cli.example.com"},
		},
		{
			name: "stdin only", stdin: "  pipe.example.com\n\tsecond.example.com\r\n\n",
			want: []string{"pipe.example.com", "second.example.com"},
		},
		{
			name: "empty stdin", wantErr: "alterx: no input found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdin, writer, err := os.Pipe()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, stdin.Close()) })
			if tt.keepOpen {
				t.Cleanup(func() { require.NoError(t, writer.Close()) })
			} else {
				_, err = writer.WriteString(tt.stdin)
				closeErr := writer.Close()
				require.NoError(t, err)
				require.NoError(t, closeErr)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			args := append([]string{"-test.run=^TestParseFlagsHelper$", "--", "-duc", "-silent"}, tt.args...)
			cmd := exec.CommandContext(ctx, os.Args[0], args...)
			// The parent owns cleanup even if the child exits fatally or times out.
			configRoot := t.TempDir()
			cmd.Env = append(os.Environ(),
				"ALTERX_TEST_PARSE_FLAGS=1",
				"XDG_CONFIG_HOME="+configRoot,
				"HOME="+configRoot,
				"home="+configRoot,
				"AppData="+configRoot,
				"USERPROFILE="+configRoot,
			)
			cmd.Stdin = stdin
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err = cmd.Run()
			require.NoError(t, ctx.Err(), "input parsing waited for an idle pipe: %s", stderr.String())
			if tt.wantErr != "" {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				require.Contains(t, stderr.String(), tt.wantErr)
				return
			}
			require.NoError(t, err, "stderr: %s", stderr.String())
			var domains []string
			require.NoError(t, json.NewDecoder(&stdout).Decode(&domains))
			require.Equal(t, tt.want, domains)
		})
	}
}

func TestParseFlagsHelper(t *testing.T) {
	if os.Getenv("ALTERX_TEST_PARSE_FLAGS") != "1" {
		return
	}
	goflags.DisableAutoConfigMigration = true
	os.Args = append([]string{"alterx-input-test"}, os.Args[3:]...)
	opts := ParseFlags()
	require.NoError(t, json.NewEncoder(os.Stdout).Encode(opts.Domains))
}
