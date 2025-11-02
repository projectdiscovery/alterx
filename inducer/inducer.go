package inducer

import (
	"fmt"

	"github.com/projectdiscovery/gologger"
)

// Config represents configuration for the pattern inducer
// This will eventually be loaded from permutations.yaml
type Config struct {
	// Future: token_dictionary, learned_patterns, etc.
	// For now, keeping minimal for tokenization phase
	MaxGroupSize int // Maximum domain group size for hierarchical partitioning (default: 5000)
}

// PatternInducer discovers patterns from subdomain enumeration results
// using the regulator algorithm with hierarchical partitioning optimization
type PatternInducer struct {
	// Configuration
	config *Config

	// Input data
	domains []string // Original domain list

	// Parsed and indexed data
	tokenized  []*TokenizedDomain      // All successfully tokenized domains
	failed     []string                // Domains that failed to tokenize
	tokenIndex *TokenIndex             // Fast token lookup index
	levelStats map[int]*LevelStats     // Statistics per level (0-indexed)
	tokenStats *TokenStats             // Global token statistics
}

// NewPatternInducer creates a new pattern inducer instance
// domains: List of domains to analyze (from Options.Domains)
// config: Configuration (can be nil for defaults)
func NewPatternInducer(domains []string, config *Config) *PatternInducer {
	if config == nil {
		config = &Config{
			MaxGroupSize: 5000, // Default from optimization strategy
		}
	}

	// Ensure max group size is reasonable
	if config.MaxGroupSize < 100 {
		config.MaxGroupSize = 100
	} else if config.MaxGroupSize > 10000 {
		config.MaxGroupSize = 10000
	}

	return &PatternInducer{
		config:     config,
		domains:    domains,
		tokenized:  make([]*TokenizedDomain, 0, len(domains)),
		failed:     make([]string, 0),
		tokenIndex: NewTokenIndex(),
		levelStats: make(map[int]*LevelStats),
		tokenStats: NewTokenStats(),
	}
}

// LoadAndTokenize processes all input domains
// Steps:
// 1. For each domain, extract subdomain part (trim root via publicsuffix)
// 2. Tokenize subdomain into structured levels and tokens
// 3. Build indices for fast lookup
// 4. Compute statistics
//
// Returns error if critical failure occurs, otherwise logs warnings for individual failures
func (pi *PatternInducer) LoadAndTokenize() error {
	if len(pi.domains) == 0 {
		return fmt.Errorf("no domains provided")
	}

	successCount := 0
	failCount := 0

	// Process each domain
	for domainIndex, domain := range pi.domains {
		tokenized, err := Tokenize(domain)
		if err != nil {
			pi.failed = append(pi.failed, domain)
			failCount++
			continue
		}

		// Skip domains with no subdomain (just root domain)
		if tokenized.Subdomain == "" {
			pi.failed = append(pi.failed, domain)
			failCount++
			continue
		}

		// Store tokenized domain
		pi.tokenized = append(pi.tokenized, tokenized)
		successCount++

		// Index all tokens
		for levelIndex, level := range tokenized.Levels {
			// Ensure level stats exist
			if pi.levelStats[levelIndex] == nil {
				pi.levelStats[levelIndex] = NewLevelStats(levelIndex)
			}

			// Update level statistics
			pi.levelStats[levelIndex].IncrementDomainCount(len(level.Tokens))

			// Index each token at this level
			for _, token := range level.Tokens {
				// Add to token index
				pi.tokenIndex.Add(levelIndex, token.Position, token.Value, domainIndex)

				// Update level stats
				pi.levelStats[levelIndex].AddToken(token.Position, token.Value)

				// Update global token stats
				pi.tokenStats.AddToken(token.Type)
			}
		}
	}

	// Compute final statistics
	pi.computeFinalStats()

	if successCount == 0 {
		return fmt.Errorf("all domains failed to tokenize")
	}

	gologger.Info().Msgf("Tokenized %d domains (%d failed)", successCount, failCount)

	return nil
}

// computeFinalStats calculates aggregate statistics after tokenization
func (pi *PatternInducer) computeFinalStats() {
	// Calculate unique tokens across all levels and positions
	uniqueTokens := make(map[string]bool)
	totalTokenCount := 0

	for _, level := range pi.levelStats {
		for token := range level.TokenCounts {
			uniqueTokens[token] = true
		}
		for _, count := range level.TokenCounts {
			totalTokenCount += count
		}
	}

	pi.tokenStats.SetUniqueTokenCount(len(uniqueTokens))

	// Calculate average tokens per level
	if len(pi.tokenized) > 0 && len(pi.levelStats) > 0 {
		avgTokens := float64(pi.tokenStats.TotalTokens) / float64(len(pi.tokenized)*len(pi.levelStats))
		pi.tokenStats.SetAvgTokensPerLevel(avgTokens)
	}
}

// GetLevel returns all tokenized data at a specific level
// levelNum: 1-indexed level number (level 1 = leftmost subdomain level)
// Returns empty slice if level doesn't exist
//
// Example: GetLevel(1) returns all level-0 (internal) data
func (pi *PatternInducer) GetLevel(levelNum int) []Level {
	if levelNum < 1 {
		return []Level{}
	}

	// Convert to 0-indexed
	levelIndex := levelNum - 1

	levels := []Level{}
	for _, tokenized := range pi.tokenized {
		level := tokenized.GetLevel(levelIndex)
		if level != nil {
			levels = append(levels, *level)
		}
	}

	return levels
}

// GetTokenizedDomain returns a specific tokenized domain by index
// Returns nil if index is out of bounds
func (pi *PatternInducer) GetTokenizedDomain(index int) *TokenizedDomain {
	if index < 0 || index >= len(pi.tokenized) {
		return nil
	}
	return pi.tokenized[index]
}

// GetTokenizedDomains returns all successfully tokenized domains
func (pi *PatternInducer) GetTokenizedDomains() []*TokenizedDomain {
	return pi.tokenized
}

// GetFailedDomains returns domains that failed to tokenize
func (pi *PatternInducer) GetFailedDomains() []string {
	return pi.failed
}

// GetTokenIndex returns the token index for advanced queries
func (pi *PatternInducer) GetTokenIndex() *TokenIndex {
	return pi.tokenIndex
}

// GetLevelStats returns statistics for a specific level (1-indexed external API)
// Returns nil if level doesn't exist
func (pi *PatternInducer) GetLevelStats(levelNum int) *LevelStats {
	if levelNum < 1 {
		return nil
	}

	// Convert to 0-indexed
	levelIndex := levelNum - 1
	return pi.levelStats[levelIndex]
}

// GetAllLevelStats returns statistics for all levels
// The returned map uses 1-indexed keys (level 1, level 2, etc.)
func (pi *PatternInducer) GetAllLevelStats() map[int]*LevelStats {
	// Convert to 1-indexed for external API
	external := make(map[int]*LevelStats)
	for levelIndex, stats := range pi.levelStats {
		external[levelIndex+1] = stats
	}
	return external
}

// Stats returns comprehensive statistics about the inducer
func (pi *PatternInducer) Stats() *InducerStats {
	// Calculate level count distribution
	levelCounts := make(map[int]int)
	maxLevels := 0

	for _, tokenized := range pi.tokenized {
		levelCount := tokenized.GetLevelCount()
		levelCounts[levelCount]++
		if levelCount > maxLevels {
			maxLevels = levelCount
		}
	}

	return &InducerStats{
		TotalDomains:     len(pi.domains),
		TokenizedDomains: len(pi.tokenized),
		FailedDomains:    len(pi.failed),
		MaxLevels:        maxLevels,
		LevelCounts:      levelCounts,
		TokenTypeStats:   pi.tokenStats.TypeDistribution,
	}
}

// GetConfig returns the inducer configuration
func (pi *PatternInducer) GetConfig() *Config {
	return pi.config
}
