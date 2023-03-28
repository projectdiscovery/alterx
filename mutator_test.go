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
	expectedCount := len(defaultPatterns) * len(defaultWordList) * len(opts.Domains)
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
	// here 4 indicates : no of patterns which don't have {{sub1}}
	// here 1 and 1 indicates no of patterns and domains which have {{sub1}}
	expectedCount2 := (4 * len(opts.Domains) * len(defaultWordList)) + (1 * 1 * len(defaultWordList))
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
