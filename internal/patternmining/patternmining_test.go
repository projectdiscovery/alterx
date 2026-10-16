package patternmining

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateFromPatterns(t *testing.T) {
	domains := []string{
		"api-prod-1.example.com",
		"api-prod-2.example.com",
		"api-prod-3.example.com",
	}
	miner := NewMiner(&Options{
		Domains:          domains,
		Target:           "example.com",
		MinDistance:      1,
		MaxDistance:      3,
		PatternThreshold: 50,
		QualityRatio:     25,
		MaxLength:        100,
	})

	res, err := miner.Mine()
	require.NoError(t, err)
	require.NotEmpty(t, res.Patterns)

	results := miner.GenerateFromPatterns(res.Patterns)
	require.NotEmpty(t, results)
}

func TestGenerateFromPatternsWithContext_Cancelled(t *testing.T) {
	miner := NewMiner(&Options{
		Target:           "example.com",
		PatternThreshold: 50,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	patterns := []string{
		"(api|dev).example.com",
	}

	results := miner.GenerateFromPatternsWithContext(ctx, patterns)
	require.Empty(t, results)
}

func TestGenerateFromPatternsWithContext_Timeout(t *testing.T) {
	miner := NewMiner(&Options{
		Target:           "example.com",
		PatternThreshold: 500,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	patterns := []string{
		"(api|dev).example.com",
	}

	results := miner.GenerateFromPatternsWithContext(ctx, patterns)
	require.Empty(t, results)
}

func TestGenerateFromPatterns_ThresholdLimit(t *testing.T) {
	// Pattern with multiple choices
	miner := NewMiner(&Options{
		Target:           "example.com",
		PatternThreshold: 3,
	})

	patterns := []string{
		"(a|b|c|d|e).example.com",
	}

	results := miner.GenerateFromPatterns(patterns)
	require.NotEmpty(t, results)
	require.LessOrEqual(t, len(results), 3)
}
