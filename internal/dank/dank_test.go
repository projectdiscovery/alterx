package dank

import (
	"context"
	"errors"
	"testing"
	"time"
)

// regex that matches strings of the form "a[0-2]" (3 strings: a0, a1, a2).
const smallRegex = "a[0-2]"

// regex that explodes: 5 alphas anywhere in a 5-char window. The fixed-length
// generation walk is bounded by the alphabet size (~39) raised to fixedLen, so
// fixedLen=6 here yields tens of millions of strings — enough that any of
// the bounded variants should bail before the unbounded one would.
const explodingRegex = "[a-z][a-z][a-z][a-z][a-z][0-9]"

func TestGenerateAtFixedLength_BackwardsCompat(t *testing.T) {
	d := NewDankEncoder(smallRegex, 16)
	got := d.GenerateAtFixedLength(2)
	want := []string{"a0", "a1", "a2"}
	if !equalStringSlices(got, want) {
		t.Fatalf("GenerateAtFixedLength(2) = %v, want %v", got, want)
	}
}

func TestGenerateAtFixedLengthWithLimit_HitsCap(t *testing.T) {
	d := NewDankEncoder(smallRegex, 16)
	got, err := d.GenerateAtFixedLengthWithLimit(2, 2)
	if !errors.Is(err, ErrResultLimitReached) {
		t.Fatalf("expected ErrResultLimitReached, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 results at the cap, got %d (%v)", len(got), got)
	}
}

func TestGenerateAtFixedLengthWithLimit_NoCap(t *testing.T) {
	d := NewDankEncoder(smallRegex, 16)
	got, err := d.GenerateAtFixedLengthWithLimit(2, 0)
	if err != nil {
		t.Fatalf("unexpected error with maxResults=0: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 results without cap, got %d (%v)", len(got), got)
	}
}

func TestGenerateAtFixedLengthWithLimit_NegativeFixedLen(t *testing.T) {
	d := NewDankEncoder(smallRegex, 16)
	got, err := d.GenerateAtFixedLengthWithLimit(-1, 10)
	if !errors.Is(err, ErrInvalidFixedLength) {
		t.Fatalf("expected ErrInvalidFixedLength for negative fixedLen, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice on validation failure, got %v", got)
	}
}

func TestGenerateAtFixedLengthWithContext_NilContext(t *testing.T) {
	d := NewDankEncoder(smallRegex, 16)
	// Passing a nil context must not panic; the public entry point normalises
	// it to context.Background() before reaching the recursive ctx.Err() call.
	got, err := d.GenerateAtFixedLengthWithContext(nil, 2, 0)
	if err != nil {
		t.Fatalf("expected nil error with nil ctx, got %v", err)
	}
	want := []string{"a0", "a1", "a2"}
	if !equalStringSlices(got, want) {
		t.Fatalf("GenerateAtFixedLengthWithContext(nil, 2, 0) = %v, want %v", got, want)
	}
}

func TestGenerateAtFixedLengthWithContext_Cancellation(t *testing.T) {
	d := NewDankEncoder(explodingRegex, 16)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	start := time.Now()
	got, err := d.GenerateAtFixedLengthWithContext(ctx, 6, 0)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("DFS did not honour context deadline (took %s)", elapsed)
	}
	// Partial result slice should still be returned and sorted.
	if !isSorted(got) {
		t.Fatalf("partial results should be sorted, got %v", got[:min(10, len(got))])
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isSorted(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
