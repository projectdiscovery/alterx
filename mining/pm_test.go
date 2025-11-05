package mining

import (
	"sort"
	"testing"

	levenshtein "github.com/ka-weihe/fast-levenshtein"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a PatternMiner with test data
func createTestPatternMiner(subdomains []string) *PatternMiner {
	p := &PatternMiner{
		subdomains:  subdomains,
		distanceMap: make(map[Edge]int),
		options: &Options{
			MinLDist: 2,
			MaxLDist: 10,
		},
	}

	// Calculate levenshtein distance between all subdomains
	for _, x := range p.subdomains {
		for _, y := range p.subdomains {
			if x == y {
				continue
			}
			edge := NewEdge(x, y)
			if _, ok := p.distanceMap[edge]; !ok {
				p.distanceMap[edge] = levenshtein.Distance(x, y)
			}
		}
	}

	return p
}

// Helper function to sort clusters for consistent comparison
func sortClusters(clusters [][]string) {
	for _, cluster := range clusters {
		sort.Strings(cluster)
	}
	sort.Slice(clusters, func(i, j int) bool {
		if len(clusters[i]) != len(clusters[j]) {
			return len(clusters[i]) > len(clusters[j])
		}
		return clusters[i][0] < clusters[j][0]
	})
}

// Helper function to check if two cluster sets are equal
func clustersEqual(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	sortClusters(a)
	sortClusters(b)

	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

// Test empty input
func TestGetClustersByLevenshteinDistance_Empty(t *testing.T) {
	p := createTestPatternMiner([]string{})
	clusters, err := p.getClustersByLevenshteinDistance(1)

	require.NoError(t, err)
	assert.Nil(t, clusters)
}

// Test single subdomain - should return empty (no non-singleton clusters)
func TestGetClustersByLevenshteinDistance_SingleSubdomain(t *testing.T) {
	p := createTestPatternMiner([]string{"api"})
	clusters, err := p.getClustersByLevenshteinDistance(1)

	require.NoError(t, err)
	assert.Empty(t, clusters, "Single subdomain should not create any clusters")
}

// Test subdomains that are far apart - no clusters should form
func TestGetClustersByLevenshteinDistance_NoSimilarSubdomains(t *testing.T) {
	p := createTestPatternMiner([]string{"api", "website", "dashboard"})

	// Verify distances are > 1
	assert.Greater(t, p.distanceMap[NewEdge("api", "website")], 1)
	assert.Greater(t, p.distanceMap[NewEdge("api", "dashboard")], 1)

	clusters, err := p.getClustersByLevenshteinDistance(1)

	require.NoError(t, err)
	assert.Empty(t, clusters, "Subdomains with distance >= k should not cluster")
}

// Test simple pair with distance 1
func TestGetClustersByLevenshteinDistance_SimplePair(t *testing.T) {
	// "api" and "api1" have distance 1
	p := createTestPatternMiner([]string{"api", "api1"})

	// Verify distance
	assert.Equal(t, 1, p.distanceMap[NewEdge("api", "api1")])

	clusters, err := p.getClustersByLevenshteinDistance(2) // k=2, so dist 1 < 2

	require.NoError(t, err)
	require.Len(t, clusters, 1, "Should create exactly one cluster")

	expected := [][]string{{"api", "api1"}}
	assert.True(t, clustersEqual(clusters, expected))
}

// Test distance boundary - dist < k (not <=)
func TestGetClustersByLevenshteinDistance_StrictLessThan(t *testing.T) {
	// "api" and "api12" have distance 2
	p := createTestPatternMiner([]string{"api", "api12"})

	// Verify distance
	assert.Equal(t, 2, p.distanceMap[NewEdge("api", "api12")])

	// With k=2, distance 2 is NOT < 2, so should not cluster
	clusters, err := p.getClustersByLevenshteinDistance(2)

	require.NoError(t, err)
	assert.Empty(t, clusters, "Distance = k should NOT cluster (requires dist < k)")

	// With k=3, distance 2 < 3, so should cluster
	clusters, err = p.getClustersByLevenshteinDistance(3)

	require.NoError(t, err)
	require.Len(t, clusters, 1)
	expected := [][]string{{"api", "api12"}}
	assert.True(t, clustersEqual(clusters, expected))
}

// Test NON-transitive behavior - key example from documentation
func TestGetClustersByLevenshteinDistance_NonTransitive(t *testing.T) {
	// Example 2 from documentation:
	// Items: {A, B, C} where A↔B=1, B↔C=1, A↔C=3
	// With k=2:
	// - Center A: {A, B}     (B dist 1 < 2, C dist 3 ≮ 2)
	// - Center B: {A, B, C}  (A dist 1 < 2, C dist 1 < 2)
	// - Center C: {B, C}     (B dist 1 < 2, A dist 3 ≮ 2)
	// Result: [{A, B, C}, {A, B}, {B, C}]

	// Using "api", "api1", "api12" to match this pattern
	p := createTestPatternMiner([]string{"api", "api1", "api12"})

	// Verify distances match the pattern
	assert.Equal(t, 1, p.distanceMap[NewEdge("api", "api1")])
	assert.Equal(t, 1, p.distanceMap[NewEdge("api1", "api12")])
	assert.Equal(t, 2, p.distanceMap[NewEdge("api", "api12")])

	clusters, err := p.getClustersByLevenshteinDistance(2)

	require.NoError(t, err)

	// Should get 3 distinct clusters
	require.Len(t, clusters, 3, "Should create three distinct clusters")

	expected := [][]string{
		{"api", "api1", "api12"}, // Center: api1
		{"api", "api1"},          // Center: api
		{"api1", "api12"},        // Center: api12
	}
	assert.True(t, clustersEqual(clusters, expected))
}

// Test multiple separate clusters
func TestGetClustersByLevenshteinDistance_MultipleClusters(t *testing.T) {
	// Two separate cluster groups (need to use more distant subdomains)
	// Cluster group 1: api, api1, api2
	// Cluster group 2: web, web1
	subdomains := []string{"api", "api1", "api2", "web", "web1"}
	p := createTestPatternMiner(subdomains)

	// Verify api and web are far apart
	assert.Greater(t, p.distanceMap[NewEdge("api", "web")], 2)

	clusters, err := p.getClustersByLevenshteinDistance(2)

	require.NoError(t, err)
	require.Greater(t, len(clusters), 0, "Should create at least one cluster")

	// All clusters should have more than 1 item
	for _, cluster := range clusters {
		assert.Greater(t, len(cluster), 1, "Each cluster should have more than 1 item")
	}
}

// Test deduplication behavior
func TestGetClustersByLevenshteinDistance_Deduplication(t *testing.T) {
	// "api" and "api1" both have dist 1
	// Both will generate the same cluster {api, api1}
	// Should deduplicate to just one cluster
	p := createTestPatternMiner([]string{"api", "api1"})

	clusters, err := p.getClustersByLevenshteinDistance(2)

	require.NoError(t, err)
	require.Len(t, clusters, 1, "Should deduplicate identical clusters")

	expected := [][]string{{"api", "api1"}}
	assert.True(t, clustersEqual(clusters, expected))
}

// Test real-world subdomain patterns
func TestGetClustersByLevenshteinDistance_RealWorld(t *testing.T) {
	subdomains := []string{
		"api", "api-v1", "api-v2",
		"staging", "staging-api", "staging-web",
		"prod", "prod-api", "prod-web",
		"dev", "dev-api",
	}
	p := createTestPatternMiner(subdomains)

	// Test with different thresholds
	for k := 3; k <= 5; k++ {
		clusters, err := p.getClustersByLevenshteinDistance(k)
		require.NoError(t, err)

		t.Logf("k=%d: found %d clusters", k, len(clusters))
		for i, cluster := range clusters {
			t.Logf("  Cluster %d (size=%d): %v", i, len(cluster), cluster)
			assert.GreaterOrEqual(t, len(cluster), 2, "Each cluster should have at least 2 items")
		}
	}
}

// Test overlapping clusters example from documentation
func TestGetClustersByLevenshteinDistance_OverlappingExample(t *testing.T) {
	// Example 3 from documentation (simplified):
	// Need items where we get overlapping clusters
	// Using "aa", "aaa", "aaaa" to get progressive distances
	subdomains := []string{"aa", "aaa", "aaaa", "aaaaa"}
	p := createTestPatternMiner(subdomains)

	// Print distances for verification
	for i := 0; i < len(subdomains); i++ {
		for j := i + 1; j < len(subdomains); j++ {
			edge := NewEdge(subdomains[i], subdomains[j])
			t.Logf("Distance %s ↔ %s = %d", subdomains[i], subdomains[j], p.distanceMap[edge])
		}
	}

	clusters, err := p.getClustersByLevenshteinDistance(2)

	require.NoError(t, err)
	t.Logf("Found %d clusters:", len(clusters))
	for i, cluster := range clusters {
		t.Logf("  Cluster %d: %v", i, cluster)
	}

	// Each cluster should have at least 2 items
	for _, cluster := range clusters {
		assert.GreaterOrEqual(t, len(cluster), 2)
	}
}

// Test Edge creation consistency
func TestGetClustersByLevenshteinDistance_EdgeConsistency(t *testing.T) {
	subdomains := []string{"abc", "abd", "xyz"}
	p := createTestPatternMiner(subdomains)

	// Verify Edge creates consistent keys regardless of order
	edge1 := NewEdge("abc", "abd")
	edge2 := NewEdge("abd", "abc")
	assert.Equal(t, edge1, edge2, "Edge should be order-independent")

	// Verify distance is stored correctly
	dist, ok := p.distanceMap[edge1]
	assert.True(t, ok, "Distance should be stored for edge")
	assert.Equal(t, 1, dist, "Distance between 'abc' and 'abd' should be 1")
}

// Benchmark the clustering algorithm
func BenchmarkGetClustersByLevenshteinDistance(b *testing.B) {
	subdomains := []string{
		"api", "api1", "api2", "api3", "api-v1", "api-v2",
		"web", "web1", "web2", "webapp", "website",
		"app", "app1", "app2", "mobile", "mobile-app",
		"dev", "dev-api", "staging", "staging-api", "prod", "prod-api",
	}
	p := createTestPatternMiner(subdomains)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.getClustersByLevenshteinDistance(3)
	}
}
