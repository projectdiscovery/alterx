package alterx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var testConfig = Config{
	Patterns: []string{
		"{{sub}}-{{word}}.{{root}}", // ex: api-prod.scanme.sh
		"{{word}}-{{sub}}.{{root}}", // ex: prod-api.scanme.sh
		"{{word}}.{{sub}}.{{root}}", // ex: prod.api.scanme.sh
		"{{sub}}.{{word}}.{{root}}", // ex: api.prod.scanme.sh
	},
	Payloads: map[string][]string{
		"word": {"dev", "lib", "prod", "stage", "wp"},
	},
}

func TestMutatorCount(t *testing.T) {
	opts := &Options{
		Domains: []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
	}
	opts.Patterns = testConfig.Patterns
	opts.Payloads = testConfig.Payloads

	expectedCount := len(opts.Patterns) * len(opts.Payloads["word"]) * len(opts.Domains)
	m, err := New(opts)
	require.Nil(t, err)
	require.EqualValues(t, expectedCount, m.EstimateCount())
}

func TestMutatorResults(t *testing.T) {
	opts := &Options{
		Domains: []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
	}
	opts.Patterns = testConfig.Patterns
	opts.Payloads = testConfig.Payloads
	m, err := New(opts)
	require.Nil(t, err)
	var buff bytes.Buffer
	err = m.ExecuteWithWriter(&buff)
	require.Nil(t, err)
	count := strings.Split(strings.TrimSpace(buff.String()), "\n")
	require.EqualValues(t, 80, len(count), buff.String())
}
