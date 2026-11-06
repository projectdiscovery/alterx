package alterx

import (
	"bytes"
	"cmp"
	"context"
	"math"
	"strings"
	"sync"
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

func TestEnrichPayloadsMultiLevelInputs(t *testing.T) {
	opts := &Options{
		Domains:  []string{"cloud.nuclei.scanme.sh", "api.scanme.sh"},
		Patterns: testConfig.Patterns,
		Payloads: map[string][]string{"word": {"dev"}},
		Enrich:   true,
	}
	m, err := New(opts)
	require.Nil(t, err)

	words := m.Options.Payloads["word"]
	require.Contains(t, words, "nuclei")
	require.Contains(t, words, "api")
	require.NotContains(t, words, "nucleiapi")
}

// setDefaultConfig replaces DefaultConfig for the duration of the test.
// Tests that use it must not call t.Parallel.
func setDefaultConfig(t *testing.T, cfg Config) {
	t.Helper()

	saved := DefaultConfig
	DefaultConfig = cfg

	t.Cleanup(func() { DefaultConfig = saved })
}

func TestEnrichDoesNotLeakAcrossNewCalls(t *testing.T) {
	setDefaultConfig(t, Config{
		Patterns: []string{"{{sub}}-{{word}}.{{suffix}}"},
		Payloads: map[string][]string{"word": {"dev"}},
	})

	_, err := New(&Options{Domains: []string{"secretproject.target-a.com"}, Enrich: true})
	require.NoError(t, err)

	m, err := New(&Options{Domains: []string{"api.target-b.com"}})
	require.NoError(t, err)

	var got []string
	for sub := range m.Execute(context.Background()) {
		got = append(got, sub)
	}

	require.Equal(t, []string{"api-dev.target-b.com"}, got)
}

func TestEnrichDoesNotLeakAcrossReusedOptions(t *testing.T) {
	setDefaultConfig(t, Config{
		Patterns: []string{"{{sub}}-{{word}}.{{suffix}}"},
		Payloads: map[string][]string{"word": {"dev"}},
	})

	opts := &Options{Enrich: true}

	opts.Domains = []string{"secretproject.target-a.com"}
	_, err := New(opts)
	require.NoError(t, err)

	opts.Domains = []string{"api.target-b.com"}
	m, err := New(opts)
	require.NoError(t, err)

	var got []string
	for sub := range m.Execute(context.Background()) {
		got = append(got, sub)
	}

	require.Equal(t, []string{"api-dev.target-b.com"}, got)
}

func TestNewDoesNotModifyOptions(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"", "default", "discover", "both"} {
		t.Run(cmp.Or(mode, "unset"), func(t *testing.T) {
			t.Parallel()

			newOptions := func() *Options {
				return &Options{
					Domains: []string{"api-1.example.com", "api-2.example.com"},
					Enrich:  true,
					Mode:    mode,
				}
			}
			opts := newOptions()

			_, err := New(opts)
			require.NoError(t, err)
			require.Equal(t, newOptions(), opts)
		})
	}
}

func TestNewDoesNotModifyCallerPayloads(t *testing.T) {
	payloads := map[string][]string{"word": {"dev", "dev"}}
	_, err := New(&Options{
		Domains:  []string{"secretproject.target-a.com"},
		Patterns: testConfig.Patterns,
		Payloads: payloads,
		Enrich:   true,
	})
	require.NoError(t, err)
	require.Equal(t, map[string][]string{"word": {"dev", "dev"}}, payloads)
}

func TestNewConcurrentEnrich(t *testing.T) {
	setDefaultConfig(t, Config{
		Patterns: []string{"{{sub}}-{{word}}.{{suffix}}"},
		Payloads: map[string][]string{"word": {"dev"}},
	})

	subs := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
	mutators := make([]*Mutator, len(subs))
	errs := make([]error, len(subs))

	var wg sync.WaitGroup
	for i, sub := range subs {
		wg.Go(func() {
			mutators[i], errs[i] = New(&Options{Domains: []string{sub + ".example.com"}, Enrich: true})
		})
	}

	wg.Wait()

	for i, sub := range subs {
		require.NoError(t, errs[i])
		require.ElementsMatch(t, []string{sub, "dev"}, mutators[i].Options.Payloads["word"], sub)
	}
}
