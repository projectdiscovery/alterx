package inducer

import (
	"github.com/projectdiscovery/gologger"
)

// ClusterConfig configures pattern clustering behavior
type ClusterConfig struct {
	Enabled         bool                      // Enable clustering (default: true)
	DistanceWeights DistanceWeights           // Feature weights for distance calculation
	APConfig        AffinityPropagationConfig // Affinity Propagation parameters
	MergeStrategy   MergeStrategy             // How to merge patterns within clusters
	MinClusterSize  int                       // Minimum cluster size to keep (default: 1)
}

// MergeStrategy defines how to merge patterns within a cluster
type MergeStrategy string

const (
	// MergeExemplar uses only the exemplar pattern (safest, no payload changes)
	MergeExemplar MergeStrategy = "exemplar"
	// MergeUnionConservative unions payloads only for tight clusters (Jaccard > 0.5)
	MergeUnionConservative MergeStrategy = "union_conservative"
	// MergeUnionAggressive unions all payloads (highest coverage, may introduce noise)
	MergeUnionAggressive MergeStrategy = "union_aggressive"
)

// DefaultClusterConfig provides sensible defaults for pattern clustering
var DefaultClusterConfig = ClusterConfig{
	Enabled:         true,
	DistanceWeights: DefaultConservativeWeights,
	APConfig:        DefaultAffinityPropagationConfig,
	MergeStrategy:   MergeUnionConservative,
	MinClusterSize:  1,
}

// ClusterPatterns performs unsupervised clustering on DSL patterns
// Returns merged patterns (one per cluster) and validation metrics
//
// Algorithm:
// 1. Build similarity matrix using structural distance metrics
// 2. Run Affinity Propagation to find clusters
// 3. Merge patterns within each cluster based on strategy
// 4. Validate clustering quality using Silhouette coefficient
//
// Returns:
//   - merged patterns (one per cluster)
//   - validation metrics
//   - error if clustering fails
func ClusterPatterns(patterns []*DSLPattern, config ClusterConfig) ([]*DSLPattern, ClusterValidationMetrics, error) {
	n := len(patterns)
	if n == 0 {
		return []*DSLPattern{}, ClusterValidationMetrics{}, nil
	}

	// Single pattern - no clustering needed
	if n == 1 {
		return patterns, ClusterValidationMetrics{NumClusters: 1}, nil
	}

	gologger.Debug().Msgf("Clustering %d patterns...", n)

	// Step 1: Build similarity matrix (convert distances to similarities)
	gologger.Debug().Msg("Building similarity matrix...")
	similarity := buildSimilarityMatrix(patterns, config.DistanceWeights)

	// Step 2: Run Affinity Propagation
	gologger.Debug().Msg("Running Affinity Propagation...")
	assignments, exemplarIndices := AffinityPropagation(similarity, config.APConfig)

	gologger.Verbose().Msgf("Affinity Propagation: %d patterns → %d clusters", n, len(exemplarIndices))

	// Step 3: Merge patterns within each cluster
	gologger.Debug().Msgf("Merging clusters using strategy: %s", config.MergeStrategy)
	clusters := buildClusters(patterns, assignments)

	// Log cluster details for debugging
	for i, cluster := range clusters {
		exemplarIdx := exemplarIndices[i]
		avgJaccard := 0.0
		if len(cluster.Patterns) > 1 {
			count := 0
			for j := 0; j < len(cluster.Patterns); j++ {
				for k := j + 1; k < len(cluster.Patterns); k++ {
					avgJaccard += JaccardDomainOverlap(
						patterns[cluster.Patterns[j]].Domains,
						patterns[cluster.Patterns[k]].Domains,
					)
					count++
				}
			}
			if count > 0 {
				avgJaccard /= float64(count)
			}
		}
		gologger.Debug().Msgf("Cluster %d: size=%d, exemplar=%d, avg_jaccard=%.2f, coverage=%d",
			i, len(cluster.Patterns), exemplarIdx, avgJaccard, patterns[exemplarIdx].Coverage)
	}

	merged := mergeAllClusters(patterns, clusters, config.MergeStrategy)

	// Filter by minimum cluster size
	if config.MinClusterSize > 1 {
		filtered := make([]*DSLPattern, 0, len(merged))
		for i, pattern := range merged {
			if len(clusters[i].Patterns) >= config.MinClusterSize {
				filtered = append(filtered, pattern)
			} else {
				gologger.Debug().Msgf("Filtered out small cluster: size=%d, template=%s", len(clusters[i].Patterns), pattern.Template)
			}
		}
		merged = filtered
	}

	// Step 4: Validate clustering quality
	distFunc := func(p1, p2 *DSLPattern) float64 {
		return StructuralPatternDistance(p1, p2, config.DistanceWeights)
	}
	metrics := ValidateClustering(patterns, assignments, distFunc)

	gologger.Verbose().Msgf("Clustering validation: %s", metrics.String())

	return merged, metrics, nil
}

// buildSimilarityMatrix creates N×N similarity matrix
// similarity[i][j] = -distance(i, j) (negated because AP expects similarity, not distance)
func buildSimilarityMatrix(patterns []*DSLPattern, weights DistanceWeights) [][]float64 {
	n := len(patterns)
	similarity := make([][]float64, n)

	for i := range similarity {
		similarity[i] = make([]float64, n)
		for j := range similarity[i] {
			if i == j {
				similarity[i][j] = 0.0 // Will be set to preference in AP
			} else {
				// Convert distance to similarity by negation
				dist := StructuralPatternDistance(patterns[i], patterns[j], weights)
				similarity[i][j] = -dist
			}
		}
	}

	return similarity
}

// mergeAllClusters merges patterns within each cluster according to the strategy
func mergeAllClusters(patterns []*DSLPattern, clusters []Cluster, strategy MergeStrategy) []*DSLPattern {
	merged := make([]*DSLPattern, len(clusters))

	for i, cluster := range clusters {
		clusterPatterns := make([]*DSLPattern, len(cluster.Patterns))
		for j, idx := range cluster.Patterns {
			clusterPatterns[j] = patterns[idx]
		}

		merged[i] = MergeCluster(clusterPatterns, strategy)
	}

	return merged
}

// MergeCluster merges patterns within a single cluster
func MergeCluster(patterns []*DSLPattern, strategy MergeStrategy) *DSLPattern {
	if len(patterns) == 0 {
		return nil
	}

	if len(patterns) == 1 {
		return patterns[0].Copy()
	}

	switch strategy {
	case MergeExemplar:
		return mergeExemplar(patterns)
	case MergeUnionConservative:
		return mergeUnionConservative(patterns)
	case MergeUnionAggressive:
		return mergeUnionAggressive(patterns)
	default:
		gologger.Warning().Msgf("Unknown merge strategy: %s, using exemplar", strategy)
		return mergeExemplar(patterns)
	}
}

// mergeExemplar uses the exemplar pattern with domain union (safest)
func mergeExemplar(patterns []*DSLPattern) *DSLPattern {
	// Find exemplar (pattern with minimum average distance to others)
	exemplar := findExemplar(patterns)

	// Union domains from all patterns in cluster
	allDomains := unionDomains(patterns)

	// Create merged pattern
	merged := exemplar.Copy()
	merged.Domains = allDomains
	merged.Coverage = len(allDomains)

	// Recalculate quality metrics based on new coverage
	merged.Ratio = estimateRatio(merged)
	merged.Confidence = calculateConfidence(merged.Coverage, merged.Ratio)

	return merged
}

// mergeUnionConservative unions payloads only for tight clusters (Jaccard > 0.5)
func mergeUnionConservative(patterns []*DSLPattern) *DSLPattern {
	// Check cluster tightness using average Jaccard similarity
	avgJaccard := calculateAvgJaccardInCluster(patterns)
	threshold := 0.5

	if avgJaccard > threshold {
		// Cluster is tight → safe to union payloads
		return mergeUnionAggressive(patterns)
	} else {
		// Cluster is loose → use exemplar only
		gologger.Debug().Msgf("Cluster too loose (Jaccard=%.2f), using exemplar merge", avgJaccard)
		return mergeExemplar(patterns)
	}
}

// mergeUnionAggressive unions all payloads (maximum coverage)
func mergeUnionAggressive(patterns []*DSLPattern) *DSLPattern {
	exemplar := findExemplar(patterns)

	// Union payloads for each variable position
	merged := exemplar.Copy()
	for varIdx := range merged.Variables {
		payloadSet := make(map[string]bool)

		// Collect all unique payloads across cluster
		for _, pattern := range patterns {
			if varIdx < len(pattern.Variables) {
				for _, payload := range pattern.Variables[varIdx].Payloads {
					payloadSet[payload] = true
				}
			}
		}

		// Convert set to slice
		payloads := make([]string, 0, len(payloadSet))
		for p := range payloadSet {
			payloads = append(payloads, p)
		}
		merged.Variables[varIdx].Payloads = payloads
	}

	// Union domains
	merged.Domains = unionDomains(patterns)
	merged.Coverage = len(merged.Domains)

	// Recalculate quality metrics
	merged.Ratio = estimateRatio(merged)
	merged.Confidence = calculateConfidence(merged.Coverage, merged.Ratio)

	return merged
}

// findExemplar finds the pattern with minimum average distance to all others
func findExemplar(patterns []*DSLPattern) *DSLPattern {
	if len(patterns) == 1 {
		return patterns[0]
	}

	minAvgDist := 1e9
	exemplarIdx := 0

	for i := range patterns {
		sum := 0.0
		for j := range patterns {
			if i != j {
				sum += StructuralPatternDistance(patterns[i], patterns[j], DefaultConservativeWeights)
			}
		}
		avgDist := sum / float64(len(patterns)-1)

		if avgDist < minAvgDist {
			minAvgDist = avgDist
			exemplarIdx = i
		}
	}

	return patterns[exemplarIdx]
}

// calculateAvgJaccardInCluster computes average domain overlap within cluster
func calculateAvgJaccardInCluster(patterns []*DSLPattern) float64 {
	if len(patterns) <= 1 {
		return 1.0
	}

	sum := 0.0
	count := 0
	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			sum += JaccardDomainOverlap(patterns[i].Domains, patterns[j].Domains)
			count++
		}
	}

	if count == 0 {
		return 1.0
	}

	return sum / float64(count)
}

// unionDomains creates a union of all domains across patterns
func unionDomains(patterns []*DSLPattern) []string {
	domainSet := make(map[string]bool)

	for _, pattern := range patterns {
		for _, domain := range pattern.Domains {
			domainSet[domain] = true
		}
	}

	domains := make([]string, 0, len(domainSet))
	for d := range domainSet {
		domains = append(domains, d)
	}

	return domains
}

// estimateRatio estimates generation ratio for a pattern
func estimateRatio(pattern *DSLPattern) float64 {
	// Calculate total possible generations
	totalGens := 1
	for _, v := range pattern.Variables {
		if v.NumberRange != nil {
			// Number range: count values in range
			rangeSize := v.NumberRange.End - v.NumberRange.Start + 1
			if rangeSize > 0 {
				totalGens *= rangeSize
			}
		} else {
			// Payload list: count payloads
			if len(v.Payloads) > 0 {
				totalGens *= len(v.Payloads)
			}
		}
	}

	// Ratio = estimated generations / observed coverage
	if pattern.Coverage == 0 {
		return 999.9
	}

	return float64(totalGens) / float64(pattern.Coverage)
}

// Copy creates a deep copy of a DSLPattern
func (p *DSLPattern) Copy() *DSLPattern {
	copied := &DSLPattern{
		Template:   p.Template,
		LevelCount: p.LevelCount,
		Coverage:   p.Coverage,
		Ratio:      p.Ratio,
		Confidence: p.Confidence,
	}

	// Copy variables
	copied.Variables = make([]DSLVariable, len(p.Variables))
	for i, v := range p.Variables {
		copied.Variables[i] = DSLVariable{
			Name: v.Name,
			Type: v.Type,
		}

		// Copy payloads
		if len(v.Payloads) > 0 {
			copied.Variables[i].Payloads = make([]string, len(v.Payloads))
			copy(copied.Variables[i].Payloads, v.Payloads)
		}

		// Copy number range
		if v.NumberRange != nil {
			copied.Variables[i].NumberRange = &NumberRange{
				Start:  v.NumberRange.Start,
				End:    v.NumberRange.End,
				Format: v.NumberRange.Format,
				Step:   v.NumberRange.Step,
				Type:   v.NumberRange.Type,
			}
		}
	}

	// Copy domains
	copied.Domains = make([]string, len(p.Domains))
	copy(copied.Domains, p.Domains)

	return copied
}
