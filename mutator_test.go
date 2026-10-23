package alterx

import (
	"bytes"
	"context"
	"math"
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
	opts.MaxSize = math.MaxInt
	m, err := New(opts)
	require.Nil(t, err)
	var buff bytes.Buffer
	err = m.ExecuteWithWriter(&buff)
	require.Nil(t, err)
	count := strings.Split(strings.TrimSpace(buff.String()), "\n")
	require.EqualValues(t, 80, len(count), buff.String())
}

func TestExecuteDiscoverHonorsCancelledContext(t *testing.T) {
	previous := DedupeResults
	DedupeResults = false
	t.Cleanup(func() {
		DedupeResults = previous
	})

	m, err := New(&Options{
		Mode: "discover",
		Domains: []string{
			"api-prod-1.example.com",
			"api-prod-2.example.com",
			"api-prod-3.example.com",
		},
		MaxSize:     math.MaxInt,
		MinDistance: 1,
		MaxDistance: 3,
	})
	require.NoError(t, err)

	mined, err := m.miner.Mine()
	require.NoError(t, err)
	require.NotEmpty(t, m.miner.GenerateFromPatterns(mined.Patterns))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var got []string
	for sub := range m.Execute(ctx) {
		got = append(got, sub)
	}
	require.Empty(t, got)
}

func TestMutatorLineLimit(t *testing.T) {
	opts := &Options{
		Domains: []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
		Limit:   5,
	}
	opts.Patterns = testConfig.Patterns
	opts.Payloads = testConfig.Payloads
	opts.MaxSize = math.MaxInt
	m, err := New(opts)
	require.Nil(t, err)
	var buff bytes.Buffer
	err = m.ExecuteWithWriter(&buff)
	require.Nil(t, err)
	lines := strings.Split(strings.TrimSpace(buff.String()), "\n")
	require.EqualValues(t, 5, len(lines))
	require.EqualValues(t, 5, m.payloadCount)
}
