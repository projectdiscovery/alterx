package inducer

import (
	"fmt"
	"testing"
)

func TestEditDistanceMemo_Distance(t *testing.T) {
	memo := NewEditDistanceMemo()

	tests := []struct {
		a        string
		b        string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"api", "web", 3},
		{"api-dev-01", "api-dev-02", 1},
		{"api-dev-01", "api-prod-01", 4}, // dev->prod is 4 edits
		{"api.staging.example.com", "api.prod.example.com", 7}, // staging->prod is 7 edits
	}

	for _, tt := range tests {
		result := memo.Distance(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("Distance(%q, %q) = %d; want %d", tt.a, tt.b, result, tt.expected)
		}

		// Verify memoization works (should be cached now)
		cached := memo.GetCachedDistance(tt.a, tt.b)
		if cached != result {
			t.Errorf("GetCachedDistance(%q, %q) = %d; want %d", tt.a, tt.b, cached, result)
		}
	}
}

func TestEditDistanceMemo_Memoization(t *testing.T) {
	memo := NewEditDistanceMemo()

	a, b := "api-dev-01", "api-dev-02"

	// First call should compute
	dist1 := memo.Distance(a, b)

	// Second call should use cache
	dist2 := memo.Distance(a, b)

	if dist1 != dist2 {
		t.Errorf("Distance results differ: %d vs %d", dist1, dist2)
	}

	// Verify it's actually cached
	if memo.Size() != 1 {
		t.Errorf("Expected 1 cached entry, got %d", memo.Size())
	}

	// Reversed order should use same cache entry
	dist3 := memo.Distance(b, a)
	if dist3 != dist1 {
		t.Errorf("Reversed distance differs: %d vs %d", dist3, dist1)
	}

	// Should still be only 1 entry (same key)
	if memo.Size() != 1 {
		t.Errorf("Expected 1 cached entry after reverse lookup, got %d", memo.Size())
	}
}

func TestEditDistanceMemo_PrecomputeDistances(t *testing.T) {
	memo := NewEditDistanceMemo()

	strings := []string{
		"api-dev-01",
		"api-dev-02",
		"api-dev-03",
		"api-prod-01",
	}

	memo.PrecomputeDistances(strings)

	// Should have N*(N-1)/2 = 4*3/2 = 6 entries
	expectedEntries := 6
	if memo.Size() != expectedEntries {
		t.Errorf("Expected %d cached entries, got %d", expectedEntries, memo.Size())
	}

	// All pairs should be cached
	for i := 0; i < len(strings); i++ {
		for j := i + 1; j < len(strings); j++ {
			cached := memo.GetCachedDistance(strings[i], strings[j])
			if cached == -1 {
				t.Errorf("Distance between %q and %q not cached", strings[i], strings[j])
			}
		}
	}
}

func TestEditDistanceMemo_Clear(t *testing.T) {
	memo := NewEditDistanceMemo()

	memo.Distance("a", "b")
	memo.Distance("c", "d")

	if memo.Size() != 2 {
		t.Errorf("Expected 2 entries before clear, got %d", memo.Size())
	}

	memo.Clear()

	if memo.Size() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", memo.Size())
	}
}

func TestEditDistanceMemo_EstimateMemoryUsage(t *testing.T) {
	memo := NewEditDistanceMemo()

	// Empty memo
	mem := memo.EstimateMemoryUsage()
	if mem != 0 {
		t.Errorf("Expected 0 bytes for empty memo, got %d", mem)
	}

	// Add some entries
	strings := []string{"api-dev-01", "api-dev-02", "api-dev-03"}
	memo.PrecomputeDistances(strings)

	mem = memo.EstimateMemoryUsage()
	if mem == 0 {
		t.Errorf("Expected non-zero memory usage after precomputation")
	}

	// Should be roughly 110 bytes per entry * 3 entries = ~330 bytes
	// Allow some variance
	if mem < 200 || mem > 500 {
		t.Errorf("Memory estimate %d seems unreasonable for 3 entries", mem)
	}
}

func TestMakeKey(t *testing.T) {
	tests := []struct {
		a, b     string
		expected string
	}{
		{"a", "b", "a:b"},
		{"b", "a", "a:b"}, // Should normalize
		{"api", "web", "api:web"},
		{"web", "api", "api:web"}, // Should normalize
		{"same", "same", "same:same"},
	}

	for _, tt := range tests {
		result := makeKey(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("makeKey(%q, %q) = %q; want %q", tt.a, tt.b, result, tt.expected)
		}
	}
}

func BenchmarkEditDistance(b *testing.B) {
	memo := NewEditDistanceMemo()
	a := "api-dev-01.staging.example.com"
	c := "api-prod-02.production.example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memo.Distance(a, c)
	}
}

func BenchmarkEditDistance_Memoized(b *testing.B) {
	memo := NewEditDistanceMemo()
	a := "api-dev-01.staging.example.com"
	c := "api-prod-02.production.example.com"

	// Pre-compute
	memo.Distance(a, c)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memo.Distance(a, c)
	}
}

func BenchmarkPrecomputeDistances(b *testing.B) {
	// Generate test data
	strings := make([]string, 100)
	for i := 0; i < 100; i++ {
		strings[i] = fmt.Sprintf("api-dev-%02d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memo := NewEditDistanceMemo()
		memo.PrecomputeDistances(strings)
	}
}
