package inducer

import (
	"sync"

	"github.com/agnivade/levenshtein"
)

// EditDistanceMemo provides memoized edit distance calculations
// This is critical for performance when clustering large datasets
// Following regulator's MEMO table approach but with bounded memory
type EditDistanceMemo struct {
	memo map[string]int // "domain1:domain2" -> distance
	mu   sync.RWMutex   // Thread-safe for parallel processing
}

// NewEditDistanceMemo creates a new memoization table
func NewEditDistanceMemo() *EditDistanceMemo {
	return &EditDistanceMemo{
		memo: make(map[string]int),
	}
}

// NewEditDistanceMemoWithCapacity creates a memo table with pre-allocated capacity
// Use this when you know the expected number of comparisons
func NewEditDistanceMemoWithCapacity(capacity int) *EditDistanceMemo {
	return &EditDistanceMemo{
		memo: make(map[string]int, capacity),
	}
}

// Distance calculates edit distance between two strings with memoization
// Uses Levenshtein distance algorithm
// Returns cached value if available, otherwise computes and caches
func (edm *EditDistanceMemo) Distance(a, b string) int {
	// Normalize key (always use lexicographic order for consistency)
	key := makeKey(a, b)

	// Check cache first (read lock)
	edm.mu.RLock()
	if dist, exists := edm.memo[key]; exists {
		edm.mu.RUnlock()
		return dist
	}
	edm.mu.RUnlock()

	// Compute distance
	dist := levenshtein.ComputeDistance(a, b)

	// Cache result (write lock)
	edm.mu.Lock()
	edm.memo[key] = dist
	edm.mu.Unlock()

	return dist
}

// makeKey creates a normalized key for two strings
// Always orders strings lexicographically so "a,b" and "b,a" use same key
func makeKey(a, b string) string {
	if a <= b {
		return a + ":" + b
	}
	return b + ":" + a
}

// Size returns the number of cached distance calculations
func (edm *EditDistanceMemo) Size() int {
	edm.mu.RLock()
	defer edm.mu.RUnlock()
	return len(edm.memo)
}

// Clear removes all cached distances (useful for memory management)
func (edm *EditDistanceMemo) Clear() {
	edm.mu.Lock()
	defer edm.mu.Unlock()
	edm.memo = make(map[string]int)
}

// PrecomputeDistances pre-calculates distances for a group of strings
// This is the regulator MEMO table building step
// For a group of N strings, this computes N*(N-1)/2 pairwise distances
// WARNING: Memory usage is O(N²) for the group, so only use with bounded groups
func (edm *EditDistanceMemo) PrecomputeDistances(strings []string) {
	n := len(strings)
	if n < 2 {
		return
	}

	// Pre-allocate if memo is empty (double-checked locking for concurrent safety)
	if len(edm.memo) == 0 {
		edm.mu.Lock()
		if len(edm.memo) == 0 {
			expectedSize := (n * (n - 1)) / 2
			edm.memo = make(map[string]int, expectedSize)
		}
		edm.mu.Unlock()
	}

	// Compute all pairwise distances
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// Distance() already handles caching and normalization
			edm.Distance(strings[i], strings[j])
		}
	}
}

// PrecomputeDistancesParallel pre-calculates distances using multiple goroutines
// This is useful for large groups where parallel computation provides speedup
// numWorkers: number of parallel workers (0 = use default based on CPU cores)
func (edm *EditDistanceMemo) PrecomputeDistancesParallel(strings []string, numWorkers int) {
	n := len(strings)
	if n < 2 {
		return
	}

	// Determine worker count
	if numWorkers <= 0 {
		numWorkers = 4 // Default to 4 workers
	}

	// Pre-allocate memo
	if len(edm.memo) == 0 {
		expectedSize := (n * (n - 1)) / 2
		edm.mu.Lock()
		edm.memo = make(map[string]int, expectedSize)
		edm.mu.Unlock()
	}

	// Create work queue
	type pair struct {
		i, j int
	}
	jobs := make(chan pair, n*n/2)
	var wg sync.WaitGroup

	// Spawn workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				edm.Distance(strings[job.i], strings[job.j])
			}
		}()
	}

	// Feed jobs
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			jobs <- pair{i, j}
		}
	}
	close(jobs)

	// Wait for completion
	wg.Wait()
}

// GetCachedDistance retrieves a cached distance without computing it
// Returns -1 if not cached
func (edm *EditDistanceMemo) GetCachedDistance(a, b string) int {
	key := makeKey(a, b)
	edm.mu.RLock()
	defer edm.mu.RUnlock()

	if dist, exists := edm.memo[key]; exists {
		return dist
	}
	return -1
}

// EstimateMemoryUsage estimates memory usage of the MEMO table
// Returns approximate bytes used
func (edm *EditDistanceMemo) EstimateMemoryUsage() int64 {
	edm.mu.RLock()
	defer edm.mu.RUnlock()

	// Rough estimate:
	// - Each map entry: ~50 bytes (key string + value int + map overhead)
	// - Key length varies but average ~30 chars per domain × 2 = 60 bytes
	// Total per entry: ~110 bytes
	return int64(len(edm.memo)) * 110
}
