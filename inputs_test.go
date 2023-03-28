package alterx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInput(t *testing.T) {
	testcases := []string{"scanme.co.uk", "https://scanme.co.uk", "scanme.co.uk:443", "https://scanme.co.uk:443"}
	expected := &Input{
		TLD:    "uk",
		ETLD:   "co.uk",
		Root:   "scanme.co.uk",
		Suffix: "scanme.co.uk",
		Sub:    "",
	}
	for _, v := range testcases {
		got, err := NewInput(v)
		require.Nilf(t, err, "failed to parse url %v", v)
		require.Equal(t, expected, got)
	}
}

func TestInputSub(t *testing.T) {
	testcases := []struct {
		url      string
		expected *Input
	}{
		{url: "something.scanme.sh", expected: &Input{TLD: "sh", ETLD: "", Root: "scanme.sh", Sub: "something", Suffix: "scanme.sh"}},
		{url: "nested.something.scanme.sh", expected: &Input{TLD: "sh", ETLD: "", Root: "scanme.sh", Sub: "nested", Suffix: "something.scanme.sh", MultiLevel: []string{"something"}}},
		{url: "nested.multilevel.scanme.co.uk", expected: &Input{TLD: "uk", ETLD: "co.uk", Root: "scanme.co.uk", Sub: "nested", Suffix: "multilevel.scanme.co.uk", MultiLevel: []string{"multilevel"}}},
		{url: "sub.level1.level2.scanme.sh", expected: &Input{TLD: "sh", ETLD: "", Root: "scanme.sh", Sub: "sub", Suffix: "level1.level2.scanme.sh", MultiLevel: []string{"level1", "level2"}}},
		{url: "scanme.sh", expected: &Input{TLD: "sh", ETLD: "", Sub: "", Suffix: "scanme.sh", Root: "scanme.sh"}},
	}
	for _, v := range testcases {
		got, err := NewInput(v.url)
		require.Nilf(t, err, "failed to parse url %v", v.url)
		require.Equal(t, v.expected, got, *v.expected)
	}
}

func TestVarCount(t *testing.T) {
	testcases := []struct {
		statement string
		count     int
	}{
		{statement: "{{sub}}.something.{{tld}}", count: 2},
		{statement: "{{sub}}.{{sub1}}.{{sub2}}.{{root}}", count: 4},
		{statement: "no variables", count: 0},
	}
	for _, v := range testcases {
		require.EqualValues(t, v.count, getVarCount(v.statement), "variable count mismatch")
	}
}

func TestExtractVar(t *testing.T) {
	// extract all variables from statement
	testcases := []struct {
		statement string
		expected  []string
	}{
		{statement: "{{sub}}.something.{{tld}}", expected: []string{"sub", "tld"}},
		{statement: "{{sub}}.{{sub1}}.{{sub2}}.{{root}}", expected: []string{"sub", "sub1", "sub2", "root"}},
		{statement: "no variables", expected: []string{}},
	}
	for _, v := range testcases {
		require.Equal(t, v.expected, getAllVars(v.statement))
	}
}
