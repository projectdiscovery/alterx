package inducer

import (
	"fmt"
	"math"

	"github.com/projectdiscovery/gologger"
)

// ClusterValidationMetrics holds unsupervised clustering quality scores
type ClusterValidationMetrics struct {
	Silhouette       float64 // [-1, 1], higher is better, >0.5 is good
	DaviesBouldin    float64 // [0, ∞), lower is better, <1.0 is good
	CalinskiHarabasz float64 // [0, ∞), higher is better
	NumClusters      int     // Number of clusters found
	NumSingletons    int     // Number of singleton clusters (size=1)
}

// String returns a formatted summary of validation metrics
func (m ClusterValidationMetrics) String() string {
	var quality string
	if m.Silhouette > 0.70 {
		quality = "strong"
	} else if m.Silhouette > 0.50 {
		quality = "reasonable"
	} else if m.Silhouette > 0.25 {
		quality = "weak"
	} else {
		quality = "poor"
	}

	return fmt.Sprintf(
		"Silhouette=%.3f (%s), Davies-Bouldin=%.3f, Calinski-Harabasz=%.1f, Clusters=%d, Singletons=%d",
		m.Silhouette, quality, m.DaviesBouldin, m.CalinskiHarabasz, m.NumClusters, m.NumSingletons,
	)
}

// ValidateClustering computes all validation metrics for a clustering result
func ValidateClustering(patterns []*DSLPattern, assignments []int, distFunc func(p1, p2 *DSLPattern) float64) ClusterValidationMetrics {
	metrics := ClusterValidationMetrics{}

	// Build clusters
	clusters := buildClusters(patterns, assignments)
	metrics.NumClusters = len(clusters)
	metrics.NumSingletons = countSingletons(clusters)

	// Can't compute metrics for single cluster or no clustering
	if len(clusters) <= 1 {
		gologger.Warning().Msg("Cannot compute clustering metrics for single cluster")
		return metrics
	}

	// Compute metrics
	metrics.Silhouette = CalculateSilhouette(patterns, assignments, distFunc)
	metrics.DaviesBouldin = CalculateDaviesBouldin(patterns, clusters, distFunc)
	metrics.CalinskiHarabasz = CalculateCalinskiHarabasz(patterns, clusters, distFunc)

	return metrics
}

// Cluster represents a group of pattern indices
type Cluster struct {
	Patterns []int // Indices into the patterns array
}

// buildClusters groups pattern indices by cluster assignment
func buildClusters(patterns []*DSLPattern, assignments []int) []Cluster {
	clusterMap := make(map[int][]int)

	for i, clusterID := range assignments {
		clusterMap[clusterID] = append(clusterMap[clusterID], i)
	}

	clusters := make([]Cluster, 0, len(clusterMap))
	for _, indices := range clusterMap {
		clusters = append(clusters, Cluster{Patterns: indices})
	}

	return clusters
}

// countSingletons counts clusters with only one member
func countSingletons(clusters []Cluster) int {
	count := 0
	for _, c := range clusters {
		if len(c.Patterns) == 1 {
			count++
		}
	}
	return count
}

// CalculateSilhouette computes the Silhouette Coefficient
//
// For each pattern i:
//
//	a(i) = average distance to patterns in same cluster
//	b(i) = average distance to patterns in nearest different cluster
//	s(i) = (b(i) - a(i)) / max(a(i), b(i))
//
// Overall silhouette = average of s(i) for all patterns
//
// Interpretation:
//
//	> 0.70: Strong clustering (tight, well-separated)
//	> 0.50: Reasonable clustering
//	> 0.25: Weak clustering
//	< 0:    Wrong clustering (patterns closer to other clusters)
func CalculateSilhouette(patterns []*DSLPattern, assignments []int, distFunc func(p1, p2 *DSLPattern) float64) float64 {
	n := len(patterns)
	if n <= 1 {
		return 0.0
	}

	// Build clusters
	clusters := buildClusters(patterns, assignments)
	if len(clusters) <= 1 {
		return 0.0
	}

	// Compute silhouette for each pattern
	silhouettes := make([]float64, n)
	for i := 0; i < n; i++ {
		// Find which cluster this pattern belongs to
		clusterID := assignments[i]
		myCluster := findCluster(clusters, clusterID, i)

		// a(i): average distance to patterns in same cluster
		a := avgIntraClusterDistance(i, myCluster, patterns, distFunc)

		// b(i): average distance to nearest different cluster
		b := avgNearestClusterDistance(i, clusterID, clusters, patterns, assignments, distFunc)

		// s(i) = (b - a) / max(a, b)
		if a == 0 && b == 0 {
			silhouettes[i] = 0.0
		} else {
			silhouettes[i] = (b - a) / math.Max(a, b)
		}
	}

	// Average silhouette
	sum := 0.0
	for _, s := range silhouettes {
		sum += s
	}

	return sum / float64(n)
}

// findCluster returns the cluster containing pattern i with the given cluster ID
func findCluster(clusters []Cluster, clusterID int, patternIdx int) Cluster {
	for _, c := range clusters {
		for _, idx := range c.Patterns {
			if idx == patternIdx {
				return c
			}
		}
	}
	// Shouldn't happen, but return empty cluster
	return Cluster{Patterns: []int{patternIdx}}
}

// avgIntraClusterDistance computes average distance from pattern i to all other patterns in its cluster
func avgIntraClusterDistance(i int, cluster Cluster, patterns []*DSLPattern, distFunc func(p1, p2 *DSLPattern) float64) float64 {
	if len(cluster.Patterns) <= 1 {
		return 0.0
	}

	sum := 0.0
	count := 0
	for _, j := range cluster.Patterns {
		if j != i {
			sum += distFunc(patterns[i], patterns[j])
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return sum / float64(count)
}

// avgNearestClusterDistance computes average distance from pattern i to patterns in nearest different cluster
func avgNearestClusterDistance(i int, myClusterID int, clusters []Cluster, patterns []*DSLPattern, assignments []int, distFunc func(p1, p2 *DSLPattern) float64) float64 {
	minAvgDist := math.Inf(1)

	// For each other cluster
	for _, cluster := range clusters {
		// Skip if this is my cluster
		if len(cluster.Patterns) > 0 && assignments[cluster.Patterns[0]] == myClusterID {
			continue
		}

		// Compute average distance to this cluster
		sum := 0.0
		count := 0
		for _, j := range cluster.Patterns {
			sum += distFunc(patterns[i], patterns[j])
			count++
		}

		if count > 0 {
			avgDist := sum / float64(count)
			if avgDist < minAvgDist {
				minAvgDist = avgDist
			}
		}
	}

	if math.IsInf(minAvgDist, 1) {
		return 0.0
	}

	return minAvgDist
}

// CalculateDaviesBouldin computes the Davies-Bouldin Index
//
// Measures the ratio of within-cluster to between-cluster distances
// Lower values indicate better clustering (< 1.0 is good)
//
// Formula:
//
//	DB = (1/k) * sum_{i=1}^k max_{j≠i} { (s_i + s_j) / d(c_i, c_j) }
//	where s_i = avg distance within cluster i
//	      d(c_i, c_j) = distance between cluster centroids
func CalculateDaviesBouldin(patterns []*DSLPattern, clusters []Cluster, distFunc func(p1, p2 *DSLPattern) float64) float64 {
	k := len(clusters)
	if k <= 1 {
		return 0.0
	}

	// Compute within-cluster scatter (s_i) for each cluster
	scatters := make([]float64, k)
	for i, cluster := range clusters {
		scatters[i] = clusterScatter(cluster, patterns, distFunc)
	}

	// For each cluster, find max similarity to other clusters
	sum := 0.0
	for i := 0; i < k; i++ {
		maxRatio := 0.0
		for j := 0; j < k; j++ {
			if i != j {
				// Distance between cluster centroids
				interDist := interClusterDistance(clusters[i], clusters[j], patterns, distFunc)
				if interDist > 0 {
					ratio := (scatters[i] + scatters[j]) / interDist
					if ratio > maxRatio {
						maxRatio = ratio
					}
				}
			}
		}
		sum += maxRatio
	}

	return sum / float64(k)
}

// clusterScatter computes average intra-cluster distance
func clusterScatter(cluster Cluster, patterns []*DSLPattern, distFunc func(p1, p2 *DSLPattern) float64) float64 {
	if len(cluster.Patterns) <= 1 {
		return 0.0
	}

	sum := 0.0
	count := 0
	for i := 0; i < len(cluster.Patterns); i++ {
		for j := i + 1; j < len(cluster.Patterns); j++ {
			sum += distFunc(patterns[cluster.Patterns[i]], patterns[cluster.Patterns[j]])
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return sum / float64(count)
}

// interClusterDistance computes average distance between two clusters
func interClusterDistance(c1, c2 Cluster, patterns []*DSLPattern, distFunc func(p1, p2 *DSLPattern) float64) float64 {
	if len(c1.Patterns) == 0 || len(c2.Patterns) == 0 {
		return math.Inf(1)
	}

	sum := 0.0
	count := 0
	for _, i := range c1.Patterns {
		for _, j := range c2.Patterns {
			sum += distFunc(patterns[i], patterns[j])
			count++
		}
	}

	if count == 0 {
		return math.Inf(1)
	}

	return sum / float64(count)
}

// CalculateCalinskiHarabasz computes the Calinski-Harabasz Index (Variance Ratio Criterion)
//
// Measures the ratio of between-cluster variance to within-cluster variance
// Higher values indicate better clustering
//
// Formula:
//
//	CH = (SSB / SSW) * ((N - k) / (k - 1))
//	where SSB = between-cluster sum of squares
//	      SSW = within-cluster sum of squares
//	      N = number of patterns
//	      k = number of clusters
func CalculateCalinskiHarabasz(patterns []*DSLPattern, clusters []Cluster, distFunc func(p1, p2 *DSLPattern) float64) float64 {
	n := len(patterns)
	k := len(clusters)

	if k <= 1 || n <= k {
		return 0.0
	}

	// This metric is tricky with arbitrary distance functions
	// We'll use a simplified version based on cluster compactness

	// Within-cluster sum of squares
	ssw := 0.0
	for _, cluster := range clusters {
		ssw += clusterCompactness(cluster, patterns, distFunc)
	}

	// Between-cluster sum of squares (approximate)
	ssb := 0.0
	for i := 0; i < k; i++ {
		for j := i + 1; j < k; j++ {
			ssb += interClusterDistance(clusters[i], clusters[j], patterns, distFunc) * float64(len(clusters[i].Patterns)*len(clusters[j].Patterns))
		}
	}

	if ssw == 0 {
		return 0.0
	}

	// CH = (SSB / SSW) * ((N - k) / (k - 1))
	return (ssb / ssw) * (float64(n-k) / float64(k-1))
}

// clusterCompactness computes sum of squared distances within a cluster
func clusterCompactness(cluster Cluster, patterns []*DSLPattern, distFunc func(p1, p2 *DSLPattern) float64) float64 {
	if len(cluster.Patterns) <= 1 {
		return 0.0
	}

	sum := 0.0
	for i := 0; i < len(cluster.Patterns); i++ {
		for j := i + 1; j < len(cluster.Patterns); j++ {
			dist := distFunc(patterns[cluster.Patterns[i]], patterns[cluster.Patterns[j]])
			sum += dist * dist
		}
	}

	return sum
}
