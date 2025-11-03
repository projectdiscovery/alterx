package inducer

import (
	"math"

	"github.com/projectdiscovery/gologger"
)

// AffinityPropagationConfig configures the affinity propagation algorithm
type AffinityPropagationConfig struct {
	MaxIterations int     // Maximum number of iterations (default: 200)
	Convergence   int     // Number of iterations without change to declare convergence (default: 15)
	Damping       float64 // Damping factor to avoid oscillations (default: 0.5, range: [0.5, 1.0))
	Preference    float64 // Self-similarity preference (higher → more clusters)
}

// DefaultAffinityPropagationConfig provides sensible defaults
var DefaultAffinityPropagationConfig = AffinityPropagationConfig{
	MaxIterations: 200,
	Convergence:   15,
	Damping:       0.5,
	Preference:    0.0, // Will be set to median similarity if 0
}

// AffinityPropagation performs affinity propagation clustering
// Returns cluster assignments (pattern index → cluster ID) and exemplar indices
//
// Algorithm:
// 1. Initialize responsibility (R) and availability (A) matrices to zero
// 2. Iteratively update R and A by passing "messages" between patterns
// 3. Converge when exemplars stabilize for 'convergence' iterations
// 4. Extract clusters from final R and A matrices
//
// Inputs:
//   - similarity: N×N matrix where similarity[i][j] = -distance(i, j)
//   - config: algorithm configuration
//
// Outputs:
//   - assignments: pattern index → cluster ID (exemplar index)
//   - exemplars: list of exemplar indices
func AffinityPropagation(similarity [][]float64, config AffinityPropagationConfig) (assignments []int, exemplars []int) {
	n := len(similarity)
	if n == 0 {
		return []int{}, []int{}
	}

	// Set preference to median similarity if not specified
	if config.Preference == 0.0 {
		config.Preference = calculateMedianSimilarity(similarity)
		gologger.Debug().Msgf("Auto-set AP preference to median similarity: %.4f", config.Preference)
	}

	// Add preference to diagonal (self-similarity)
	S := make([][]float64, n)
	for i := range S {
		S[i] = make([]float64, n)
		for j := range S[i] {
			if i == j {
				S[i][j] = config.Preference
			} else {
				S[i][j] = similarity[i][j]
			}
		}
	}

	// Initialize responsibility and availability matrices
	R := make([][]float64, n)
	A := make([][]float64, n)
	for i := range R {
		R[i] = make([]float64, n)
		A[i] = make([]float64, n)
	}

	// Track convergence
	prevExemplars := make([]int, 0)
	unchangedCount := 0

	// Iterate
	for iter := 0; iter < config.MaxIterations; iter++ {
		// Update responsibility
		R = updateResponsibility(S, A, R, config.Damping)

		// Update availability
		A = updateAvailability(R, A, config.Damping)

		// Check convergence
		currentExemplars := extractExemplars(R, A)
		if exemplarsEqual(prevExemplars, currentExemplars) {
			unchangedCount++
			if unchangedCount >= config.Convergence {
				gologger.Debug().Msgf("AP converged after %d iterations", iter+1)
				break
			}
		} else {
			unchangedCount = 0
		}
		prevExemplars = currentExemplars

		if (iter+1)%50 == 0 {
			gologger.Debug().Msgf("AP iteration %d/%d, current exemplars: %d", iter+1, config.MaxIterations, len(currentExemplars))
		}
	}

	// Extract final clusters
	exemplars = extractExemplars(R, A)
	assignments = assignToClusters(S, exemplars)

	gologger.Verbose().Msgf("Affinity Propagation: %d patterns → %d clusters", n, len(exemplars))

	return assignments, exemplars
}

// updateResponsibility updates the responsibility matrix
// r(i,k) = s(i,k) - max{a(i,k') + s(i,k')} for k' ≠ k
// Interpretation: How well-suited k is to be the exemplar for i
func updateResponsibility(S, A, oldR [][]float64, damping float64) [][]float64 {
	n := len(S)
	newR := make([][]float64, n)

	for i := range newR {
		newR[i] = make([]float64, n)

		// For each potential exemplar k
		for k := range newR[i] {
			// Find max{a(i,k') + s(i,k')} for k' ≠ k
			maxVal := math.Inf(-1)
			for kPrime := 0; kPrime < n; kPrime++ {
				if kPrime != k {
					val := A[i][kPrime] + S[i][kPrime]
					if val > maxVal {
						maxVal = val
					}
				}
			}

			// r(i,k) = s(i,k) - max{a(i,k') + s(i,k')}
			newR[i][k] = S[i][k] - maxVal

			// Apply damping to avoid oscillations
			newR[i][k] = damping*oldR[i][k] + (1-damping)*newR[i][k]
		}
	}

	return newR
}

// updateAvailability updates the availability matrix
// a(i,k) = min{0, r(k,k) + sum{max(0, r(i',k))} for i' ∉ {i,k}}  (for i ≠ k)
// a(k,k) = sum{max(0, r(i',k))} for i' ≠ k                        (for i = k)
// Interpretation: How appropriate it is for i to choose k as exemplar
func updateAvailability(R, oldA [][]float64, damping float64) [][]float64 {
	n := len(R)
	newA := make([][]float64, n)

	for i := range newA {
		newA[i] = make([]float64, n)

		for k := range newA[i] {
			if i == k {
				// Self-availability: a(k,k) = sum{max(0, r(i',k))} for i' ≠ k
				sum := 0.0
				for iPrime := 0; iPrime < n; iPrime++ {
					if iPrime != k {
						sum += math.Max(0, R[iPrime][k])
					}
				}
				newA[i][k] = sum
			} else {
				// a(i,k) = min{0, r(k,k) + sum{max(0, r(i',k))}}
				sum := R[k][k]
				for iPrime := 0; iPrime < n; iPrime++ {
					if iPrime != i && iPrime != k {
						sum += math.Max(0, R[iPrime][k])
					}
				}
				newA[i][k] = math.Min(0, sum)
			}

			// Apply damping
			newA[i][k] = damping*oldA[i][k] + (1-damping)*newA[i][k]
		}
	}

	return newA
}

// extractExemplars identifies exemplar indices where r(i,i) + a(i,i) > 0
func extractExemplars(R, A [][]float64) []int {
	exemplars := []int{}

	for i := range R {
		if R[i][i]+A[i][i] > 0 {
			exemplars = append(exemplars, i)
		}
	}

	// If no exemplars found, pick the pattern with highest self-responsibility
	if len(exemplars) == 0 {
		maxIdx := 0
		maxVal := R[0][0] + A[0][0]
		for i := 1; i < len(R); i++ {
			val := R[i][i] + A[i][i]
			if val > maxVal {
				maxVal = val
				maxIdx = i
			}
		}
		exemplars = append(exemplars, maxIdx)
	}

	return exemplars
}

// assignToClusters assigns each pattern to its nearest exemplar
func assignToClusters(S [][]float64, exemplars []int) []int {
	n := len(S)
	assignments := make([]int, n)

	for i := 0; i < n; i++ {
		// Find exemplar with highest similarity
		bestExemplar := exemplars[0]
		bestSim := S[i][exemplars[0]]

		for _, ex := range exemplars[1:] {
			if S[i][ex] > bestSim {
				bestSim = S[i][ex]
				bestExemplar = ex
			}
		}

		assignments[i] = bestExemplar
	}

	return assignments
}

// exemplarsEqual checks if two exemplar sets are identical
func exemplarsEqual(e1, e2 []int) bool {
	if len(e1) != len(e2) {
		return false
	}

	// Convert to sets
	set1 := make(map[int]bool)
	for _, e := range e1 {
		set1[e] = true
	}

	for _, e := range e2 {
		if !set1[e] {
			return false
		}
	}

	return true
}

// calculateMedianSimilarity computes the median of all similarity values
func calculateMedianSimilarity(similarity [][]float64) float64 {
	n := len(similarity)
	if n == 0 {
		return 0.0
	}

	// Collect all non-diagonal similarities
	values := make([]float64, 0, n*(n-1))
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j {
				values = append(values, similarity[i][j])
			}
		}
	}

	if len(values) == 0 {
		return 0.0
	}

	// Sort to find median
	sortFloat64s(values)

	mid := len(values) / 2
	if len(values)%2 == 0 {
		return (values[mid-1] + values[mid]) / 2.0
	}
	return values[mid]
}

// sortFloat64s is a simple bubble sort for float64 slices
// (Go's sort.Float64s can't be used since we need full control)
func sortFloat64s(arr []float64) {
	n := len(arr)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if arr[j] > arr[j+1] {
				arr[j], arr[j+1] = arr[j+1], arr[j]
			}
		}
	}
}
