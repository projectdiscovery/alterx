package inducer

// InductionMode represents the execution mode based on input size
type InductionMode int

const (
	// ModeThorough provides highest accuracy for small datasets (<100 domains)
	ModeThorough InductionMode = iota
	// ModeBalanced balances accuracy and speed (100-1000 domains)
	ModeBalanced
	// ModeFast optimizes for performance on large datasets (>1000 domains)
	ModeFast
)

// String returns mode name
func (m InductionMode) String() string {
	switch m {
	case ModeThorough:
		return "THOROUGH"
	case ModeBalanced:
		return "BALANCED"
	case ModeFast:
		return "FAST"
	default:
		return "UNKNOWN"
	}
}

// ModeConfig contains all mode-specific parameters
type ModeConfig struct {
	Mode InductionMode

	// Coverage targets
	TargetCoverage   float64 // 0.95, 0.90, 0.85
	ElbowSensitivity float64 // 0.01, 0.02, 0.03

	// Pattern bounds
	MinPatterns int // 8, 5, 3
	MaxPatterns int // 30, 25, 20

	// Clustering parameters
	DistLow  int     // 2, 2, 2
	DistHigh int     // 8, 6, 4
	MaxRatio float64 // 18.0, 15.0, 12.0

	// AP clustering
	APIterations int // 12, 10, 6

	// Enrichment
	EnrichmentRate float64 // 0.80, 0.50, 0.50

	// Strategy 2 (N-gram) configuration
	EnableNgramStrategy bool // false, true, true
	NgramThreshold      int  // 0, 200, 100

	// Token limiting (FAST only)
	EnableTokenLimiting bool // false, false, true
	MaxTokenGroups      int  // 0, 0, 30

	// Group sampling (FAST only)
	EnableGroupSampling bool // false, false, true
	MaxGroupSize        int  // 0, 0, 500
}

// DetectMode determines the appropriate mode based on input size
func DetectMode(inputSize int) InductionMode {
	if inputSize < 100 {
		return ModeThorough
	}
	if inputSize <= 1000 {
		return ModeBalanced
	}
	return ModeFast
}

// NewModeConfig creates configuration for the detected mode
func NewModeConfig(inputSize int) *ModeConfig {
	mode := DetectMode(inputSize)

	switch mode {
	case ModeThorough:
		return &ModeConfig{
			Mode:                ModeThorough,
			TargetCoverage:      0.95,
			ElbowSensitivity:    0.01,
			MinPatterns:         8,
			MaxPatterns:         30,
			DistLow:             2,
			DistHigh:            8,
			MaxRatio:            18.0,
			APIterations:        12,
			EnrichmentRate:      0.80,
			EnableNgramStrategy: false, // Disabled - overhead not worth it
			NgramThreshold:      0,
			EnableTokenLimiting: false,
			MaxTokenGroups:      0,
			EnableGroupSampling: false,
			MaxGroupSize:        0,
		}

	case ModeBalanced:
		return &ModeConfig{
			Mode:                ModeBalanced,
			TargetCoverage:      0.90,
			ElbowSensitivity:    0.02,
			MinPatterns:         5,
			MaxPatterns:         25,
			DistLow:             2,
			DistHigh:            6,
			MaxRatio:            15.0,
			APIterations:        10,
			EnrichmentRate:      0.50,
			EnableNgramStrategy: true,
			NgramThreshold:      200, // Only if group > 200
			EnableTokenLimiting: false,
			MaxTokenGroups:      0,
			EnableGroupSampling: false,
			MaxGroupSize:        0,
		}

	case ModeFast:
		return &ModeConfig{
			Mode:                ModeFast,
			TargetCoverage:      0.85,
			ElbowSensitivity:    0.03,
			MinPatterns:         3,
			MaxPatterns:         20,
			DistLow:             2,
			DistHigh:            4,
			MaxRatio:            12.0,
			APIterations:        6,
			EnrichmentRate:      0.50,
			EnableNgramStrategy: true,
			NgramThreshold:      100, // Only if group > 100
			EnableTokenLimiting: true,
			MaxTokenGroups:      30,
			EnableGroupSampling: true,
			MaxGroupSize:        500,
		}

	default:
		// Fallback to balanced
		return NewModeConfig(500)
	}
}
