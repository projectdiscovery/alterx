package dank

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateAtFixedLength(t *testing.T) {
	encoder := NewDankEncoder("api(1|2)", 4)
	results := encoder.GenerateAtFixedLength(4)
	expected := []string{"api1", "api2"}
	require.Equal(t, expected, results)

	encoderRange := NewDankEncoder("web[1-3]", 4)
	resultsRange := encoderRange.GenerateAtFixedLength(4)
	expectedRange := []string{"web1", "web2", "web3"}
	require.Equal(t, expectedRange, resultsRange)
}

func TestGenerateAtFixedLengthWithLimit(t *testing.T) {
	encoder := NewDankEncoder("node[1-9]", 5)

	allResults := encoder.GenerateAtFixedLength(5)
	require.Len(t, allResults, 9)

	// Truncated to 3 results
	limited := encoder.GenerateAtFixedLengthWithLimit(5, 3)
	require.Len(t, limited, 3)
	require.Equal(t, allResults[:3], limited)

	// Single result limit
	single := encoder.GenerateAtFixedLengthWithLimit(5, 1)
	require.Len(t, single, 1)
	require.Equal(t, allResults[:1], single)

	// Limit equal to total results
	exact := encoder.GenerateAtFixedLengthWithLimit(5, 9)
	require.Len(t, exact, 9)
	require.Equal(t, allResults, exact)

	// Limit greater than total results
	oversized := encoder.GenerateAtFixedLengthWithLimit(5, 20)
	require.Len(t, oversized, 9)
	require.Equal(t, allResults, oversized)

	// Non-positive limits indicate no limit
	unlimitedZero := encoder.GenerateAtFixedLengthWithLimit(5, 0)
	require.Equal(t, allResults, unlimitedZero)

	unlimitedNegative := encoder.GenerateAtFixedLengthWithLimit(5, -1)
	require.Equal(t, allResults, unlimitedNegative)
}

func TestGenerateAtFixedLengthWithLimitDeepRecursion(t *testing.T) {
	// (a|b|c|d|e)* at length 20 contains 5^20 permutations
	// DFS without limit would run indefinitely
	encoder := NewDankEncoder("(a|b|c|d|e)*", 20)

	start := time.Now()
	results := encoder.GenerateAtFixedLengthWithLimit(20, 10)
	elapsed := time.Since(start)

	require.Len(t, results, 10)
	require.Less(t, elapsed, 2*time.Second)

	for _, item := range results {
		require.Len(t, item, 20)
	}

	isSorted := sort.StringsAreSorted(results)
	require.True(t, isSorted)
}

func TestGenerateAtFixedLengthWithContext(t *testing.T) {
	encoder := NewDankEncoder("api(1|2)", 4)

	// Success path with active context
	ctx := context.Background()
	results, err := encoder.GenerateAtFixedLengthWithContext(ctx, 4)
	require.NoError(t, err)
	require.Equal(t, []string{"api1", "api2"}, results)

	// Pre-cancelled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	canceledResults, err := encoder.GenerateAtFixedLengthWithContext(canceledCtx, 4)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
	require.Nil(t, canceledResults)

	// Nil context falls back to background
	nilCtxResults, err := encoder.GenerateAtFixedLengthWithContext(nil, 4)
	require.NoError(t, err)
	require.Equal(t, []string{"api1", "api2"}, nilCtxResults)
}

func TestGenerateAtFixedLengthWithContextTimeout(t *testing.T) {
	// (a|b|c|d|e)* at length 30 has 5^30 combinations
	encoder := NewDankEncoder("(a|b|c|d|e)*", 30)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	results, err := encoder.GenerateAtFixedLengthWithContext(ctx, 30)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded))
	require.Nil(t, results)
	require.Less(t, elapsed, 1*time.Second)
}

func TestGenerateAtFixedLengthWithContextAndLimit(t *testing.T) {
	encoder := NewDankEncoder("(a|b|c|d|e)*", 20)

	// Limit reached before timeout expires
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := encoder.GenerateAtFixedLengthWithContextAndLimit(ctx, 20, 5)
	require.NoError(t, err)
	require.Len(t, results, 5)

	// Pre-cancelled context with limit
	canceledCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()

	canceledResults, err := encoder.GenerateAtFixedLengthWithContextAndLimit(canceledCtx, 20, 5)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
	require.Nil(t, canceledResults)
}

func TestGenerateAtFixedLengthEdgeCases(t *testing.T) {
	encoder := NewDankEncoder("test", 4)

	// Negative length
	negResults := encoder.GenerateAtFixedLength(-1)
	require.Empty(t, negResults)

	negContextResults, err := encoder.GenerateAtFixedLengthWithContext(context.Background(), -1)
	require.NoError(t, err)
	require.Empty(t, negContextResults)

	// Length with no matching paths
	noMatchResults := encoder.GenerateAtFixedLength(10)
	require.Empty(t, noMatchResults)
}
