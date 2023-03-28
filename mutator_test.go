package alterx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMutatorCount(t *testing.T) {
	opts := &Options{
		Domains: []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
	}
	// Default patterns:
	// {{sub}}-{{word}}.{{suffix}}
	// {{word}}-{{sub}}.{{suffix}}
	// {{word}}.{{sub}}.{{suffix}}
	// {{sub}}.{{word}}.{{suffix}}
	// Here len(DefaultWordList["word"]) used since in default patterns ^
	// there's only one different-var(which can't derived from domain) is used which is {{word}}.
	// If the pattern is '{{sub}}.{{word}}.{{year}}.{{suffix}}' then
	// expectedCount = len(Patterns) * (len(WordList["word"])+len(WordList["year"]))* len(opts.Domains)
	expectedCount := len(defaultPatterns) * len(defaultWordList["word"]) * len(opts.Domains)
	m, err := New(opts)
	require.Nil(t, err)
	require.EqualValues(t, expectedCount, m.EstimateCount())

	// advanced use case only (mutator only executes template if all variable data is available)
	opts.Patterns = []string{
		"{{sub}}-{{word}}.{{root}}",          // ex: api-prod.scanme.sh
		"{{word}}-{{sub}}.{{root}}",          // ex: prod-api.scanme.sh
		"{{word}}.{{sub}}.{{root}}",          // ex: prod.api.scanme.sh
		"{{sub}}.{{word}}.{{root}}",          // ex: api.prod.scanme.sh
		"{{sub}}.{{word}}-{{sub1}}.{{root}}", // ex: cloud.nuclei-dev.scanme.sh
	}
	// in this case count will be totally different since ^(comment)
	// here 3 indicates : no of domains which don't have {{sub1}}
	// here 1 indicates : no of patterns which have {{sub1}}
	expectedCount2 := (len(opts.Domains)*len(opts.Patterns))*(len(defaultWordList["word"])) - (3 * 1 * len(defaultWordList["word"]))
	m, err = New(opts)
	require.Nil(t, err)
	require.EqualValues(t, expectedCount2, m.EstimateCount())
}

func TestMutatorResults(t *testing.T) {
	opts := &Options{
		Domains: []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
	}
	m, err := New(opts)
	require.Nil(t, err)
	var buff bytes.Buffer
	err = m.ExecuteWithWriter(&buff)
	require.Nil(t, err)
	count := strings.Split(strings.TrimSpace(buff.String()), "\n")
	require.EqualValues(t, 80, len(count), buff.String())
}
