package inducer

import (
	"fmt"
	"math"

	"github.com/projectdiscovery/gologger"
)

// APAutoTuner configures automatic tuning of Affinity Propagation preference parameter
type APAutoTuner struct {
	// TargetClusters is the desired number of clusters
	TargetClusters int

	// MaxIterations limits binary search iterations (default: 7)
	MaxIterations int

	// MinSilhouette is the minimum acceptable clustering quality (default: 0.25)
	// Silhouette interpretation:
	//   > 0.70: Strong clustering
	//   > 0.50: Reasonable clustering
	//   > 0.25: Weak but acceptable clustering
	//   <  0:   Poor clustering (patterns closer to other clusters)
	MinSilhouette float64

	// Tolerance allows deviation from target (default: 1)
	// Result is considered converged if |actual - target| <= tolerance
	Tolerance int

	// PreferenceRange defines the search space [low, high] (default: [-2.0, 0.0])
	// Higher values (closer to 0) produce more clusters
	// Lower values (more negative) produce fewer clusters
	PreferenceRange [2]float64
}

// AutoTuneResult contains the results of auto-tuning
type AutoTuneResult struct {
	// OptimalPreference is the best preference value found
	OptimalPreference float64

	// ActualClusters is the number of clusters produced by optimal preference
	ActualClusters int

	// Silhouette is the clustering quality metric
	Silhouette float64

	// Iterations is the number of binary search iterations used
	Iterations int

	// Converged indicates whether search successfully converged
	Converged bool

	// Deviation is the absolute difference from target
	Deviation int
}

// DefaultAPAutoTuner creates an auto-tuner with reasonable defaults
func DefaultAPAutoTuner(targetClusters int) *APAutoTuner {
	return &APAutoTuner{
		TargetClusters:  targetClusters,
		MaxIterations:   7,
		MinSilhouette:   0.25,
		Tolerance:       1,
		PreferenceRange: [2]float64{-2.0, 0.0},
	}
}

// FindOptimalPreference uses binary search to find the AP preference that
// produces a cluster count closest to the target while maintaining quality
func (apt *APAutoTuner) FindOptimalPreference(
	patterns []*DSLPattern,
	config ClusterConfig,
) (AutoTuneResult, error) {

	if len(patterns) == 0 {
		return AutoTuneResult{}, fmt.Errorf("no patterns to cluster")
	}

	if len(patterns) <= apt.TargetClusters {
		// Already at or below target, no clustering needed
		medianSim := computeMedianSimilarityFallback(patterns, config.DistanceWeights)
		return AutoTuneResult{
			OptimalPreference: medianSim,
			ActualClusters:    len(patterns),
			Silhouette:        1.0, // Perfect when each pattern is its own cluster
			Iterations:        0,
			Converged:         true,
			Deviation:         int(math.Abs(float64(len(patterns) - apt.TargetClusters))),
		}, nil
	}

	gologger.Verbose().Msgf("Auto-tuning AP preference: %d patterns → target %d clusters",
		len(patterns), apt.TargetClusters)

	// Binary search state
	prefLow := apt.PreferenceRange[0]
	prefHigh := apt.PreferenceRange[1]

	// Track best result
	var bestResult *AutoTuneResult
	minDeviation := math.MaxInt32

	// Binary search
	for iter := 1; iter <= apt.MaxIterations; iter++ {
		prefMid := (prefLow + prefHigh) / 2.0

		// Run clustering with this preference
		testConfig := config // Copy config
		testConfig.APConfig.Preference = prefMid

		clustered, metrics, err := ClusterPatterns(patterns, testConfig)
		if err != nil {
			gologger.Debug().Msgf("AP tuning iter %d: clustering failed at pref=%.4f: %v",
				iter, prefMid, err)
			// Try moving towards higher preference (more clusters)
			prefLow = prefMid
			continue
		}

		actualClusters := len(clustered)
		silhouette := metrics.Silhouette
		deviation := int(math.Abs(float64(actualClusters - apt.TargetClusters)))

		gologger.Debug().Msgf("AP tuning iter %d: pref=%.4f → %d clusters (silhouette=%.3f, dev=%d)",
			iter, prefMid, actualClusters, silhouette, deviation)

		// Track best result that meets quality constraint
		if silhouette >= apt.MinSilhouette && deviation < minDeviation {
			minDeviation = deviation
			bestResult = &AutoTuneResult{
				OptimalPreference: prefMid,
				ActualClusters:    actualClusters,
				Silhouette:        silhouette,
				Iterations:        iter,
				Converged:         deviation <= apt.Tolerance,
				Deviation:         deviation,
			}
		}

		// Check convergence
		if deviation <= apt.Tolerance && silhouette >= apt.MinSilhouette {
			gologger.Debug().Msgf("AP tuning converged at iter %d", iter)
			bestResult.Converged = true
			break
		}

		// Adjust search range based on cluster count
		if actualClusters < apt.TargetClusters {
			// Need more clusters, increase preference (move towards 0)
			prefLow = prefMid
		} else if actualClusters > apt.TargetClusters {
			// Need fewer clusters, decrease preference (move away from 0)
			prefHigh = prefMid
		} else {
			// Exact match but might not meet quality
			if silhouette < apt.MinSilhouette {
				prefLow = prefMid // Try higher preference for better separation
			} else {
				bestResult.Converged = true
				break
			}
		}

		// Check if range is too narrow
		if math.Abs(prefHigh-prefLow) < 0.0001 {
			gologger.Debug().Msgf("AP tuning: search range collapsed at iter %d", iter)
			break
		}
	}

	// Return best result found
	if bestResult == nil {
		// Fallback: use median similarity
		medianSim := computeMedianSimilarityFallback(patterns, config.DistanceWeights)
		gologger.Warning().Msgf("AP tuning failed to find valid solution, using median similarity: %.4f", medianSim)
		return AutoTuneResult{
			OptimalPreference: medianSim,
			ActualClusters:    len(patterns), // Unknown, will cluster with this
			Silhouette:        0.0,
			Iterations:        apt.MaxIterations,
			Converged:         false,
			Deviation:         math.MaxInt32,
		}, nil
	}

	gologger.Verbose().Msgf("AP tuning complete: pref=%.4f → %d clusters (silhouette=%.3f) in %d iterations",
		bestResult.OptimalPreference, bestResult.ActualClusters, bestResult.Silhouette, bestResult.Iterations)

	return *bestResult, nil
}

// computeMedianSimilarityFallback computes the median similarity between all pattern pairs
// This is a reasonable default preference when tuning fails
func computeMedianSimilarityFallback(patterns []*DSLPattern, weights DistanceWeights) float64 {
	if len(patterns) < 2 {
		return -0.5 // Default fallback
	}

	// Sample similarity values (don't compute full N² matrix for large datasets)
	sampleSize := len(patterns)
	if sampleSize > 100 {
		sampleSize = 100
	}
	similarities := []float64{}

	for i := 0; i < sampleSize-1; i++ {
		for j := i + 1; j < sampleSize; j++ {
			dist := StructuralPatternDistance(patterns[i], patterns[j], weights)
			// Convert distance [0,1] to similarity [-1, 0]
			sim := -dist
			similarities = append(similarities, sim)
		}
	}

	if len(similarities) == 0 {
		return -0.5
	}

	// Calculate median
	sorted := make([]float64, len(similarities))
	copy(sorted, similarities)
	// Simple sort for median calculation
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	median := sorted[len(sorted)/2]
	return median
}
