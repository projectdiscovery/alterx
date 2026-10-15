package dank

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateAtFixedLength(t *testing.T) {
	encoder := NewDankEncoder("(a|b)(1|2)", 2)
	results := encoder.GenerateAtFixedLength(2)

	expected := []string{"a1", "a2", "b1", "b2"}
	require.Equal(t, expected, results)
}

func TestGenerateAtFixedLength_NegativeLength(t *testing.T) {
	encoder := NewDankEncoder("(a|b)(1|2)", 2)
	results := encoder.GenerateAtFixedLength(-1)
	require.Empty(t, results)
}

func TestGenerateAtFixedLengthWithLimit(t *testing.T) {
	// (a|b|c|d)[0-9] produces 40 strings of length 2
	encoder := NewDankEncoder("(a|b|c|d)[0-9]", 2)

	// Test limit = 5
	limited5 := encoder.GenerateAtFixedLengthWithLimit(2, 5)
	require.Len(t, limited5, 5)
	require.Equal(t, []string{"a0", "a1", "a2", "a3", "a4"}, limited5)

	// Test limit = 10
	limited10 := encoder.GenerateAtFixedLengthWithLimit(2, 10)
	require.Len(t, limited10, 10)

	// Test limit larger than possible (e.g. 100)
	limited100 := encoder.GenerateAtFixedLengthWithLimit(2, 100)
	require.Len(t, limited100, 40)

	// Test limit = 0 (unlimited)
	unlimited := encoder.GenerateAtFixedLengthWithLimit(2, 0)
	require.Len(t, unlimited, 40)

	// Test limit < 0 (unlimited)
	negLimit := encoder.GenerateAtFixedLengthWithLimit(2, -5)
	require.Len(t, negLimit, 40)
}

func TestGenerateAtFixedLengthWithContext_Cancelled(t *testing.T) {
	encoder := NewDankEncoder("(a|b|c|d)[0-9]", 2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	results, err := encoder.GenerateAtFixedLengthWithContext(ctx, 2)
	require.Error(t, err)
	require.Equal(t, context.Canceled, err)
	require.Empty(t, results)
}

func TestGenerateAtFixedLengthWithContext_Timeout(t *testing.T) {
	// Create a pattern with large branching factor
	encoder := NewDankEncoder("[a-z][a-z][a-z][a-z][a-z]", 5)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // Ensure timeout is expired

	_, err := encoder.GenerateAtFixedLengthWithContext(ctx, 5)
	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, err)
}

func TestGenerateAtFixedLengthWithContextAndLimit(t *testing.T) {
	encoder := NewDankEncoder("(a|b|c|d)[0-9]", 2)

	// Normal execution with limit
	ctx := context.Background()
	results, err := encoder.GenerateAtFixedLengthWithContextAndLimit(ctx, 2, 7)
	require.NoError(t, err)
	require.Len(t, results, 7)

	// Cancelled context with limit
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err = encoder.GenerateAtFixedLengthWithContextAndLimit(canceledCtx, 2, 7)
	require.Error(t, err)
	require.Equal(t, context.Canceled, err)
	require.Empty(t, results)
}
