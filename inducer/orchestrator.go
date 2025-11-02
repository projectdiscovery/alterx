package inducer

import (
	"fmt"

	"github.com/projectdiscovery/gologger"
)

// OrchestratorConfig contains configuration for the pattern induction orchestrator
type OrchestratorConfig struct {
	MaxGroupSize      int              // Maximum domains per group (default: 5000)
	RootDomain        string           // Root domain for pattern generation
	ClusteringConfig  *ClusteringConfig // Clustering configuration
	QualityConfig     *QualityConfig    // Quality filtering configuration
	EnableCompression bool             // Enable number range compression (default: true)
	EnableDedupe      bool             // Enable pattern deduplication (default: true)
}

// DefaultOrchestratorConfig returns sensible defaults
func DefaultOrchestratorConfig(rootDomain string) *OrchestratorConfig {
	return &OrchestratorConfig{
		MaxGroupSize:      5000,
		RootDomain:        rootDomain,
		ClusteringConfig:  DefaultClusteringConfig(),
		QualityConfig:     DefaultQualityConfig(),
		EnableCompression: true,
		EnableDedupe:      true,
	}
}

// Orchestrator coordinates the entire pattern induction pipeline
// This is the main entry point for pattern learning
type Orchestrator struct {
	config        *OrchestratorConfig
	partitioner   *Partitioner
	clusterer     *Clusterer
	generator     *PatternGenerator
	compressor    *NumberCompressor
	qualityFilter *QualityFilter
}

// NewOrchestrator creates a new pattern induction orchestrator
func NewOrchestrator(config *OrchestratorConfig) *Orchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig("")
	}

	return &Orchestrator{
		config:        config,
		partitioner:   NewPartitioner(config.MaxGroupSize),
		clusterer:     NewClusterer(config.ClusteringConfig),
		generator:     NewPatternGenerator(config.RootDomain),
		compressor:    NewNumberCompressor(),
		qualityFilter: NewQualityFilter(config.QualityConfig),
	}
}

// LearnPatterns executes the complete pattern induction pipeline
// This implements the full algorithm from literature_survey/proposed_solution.md
//
// Pipeline stages:
// 1. Hierarchical prefix partitioning (bounded groups)
// 2. Per-group clustering with edit distance
// 3. Pattern generation from closures
// 4. Number range compression
// 5. Quality filtering
// 6. Deduplication
//
// Returns learned patterns and any errors
func (o *Orchestrator) LearnPatterns(domains []string) ([]*Pattern, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains provided")
	}

	// Stage 1: Hierarchical Prefix Partitioning
	groups := o.partitioner.Partition(domains)

	// Stage 2-3: Per-group clustering and pattern generation
	allPatterns := []*Pattern{}

	for _, group := range groups {
		// Cluster within group
		closures := o.clusterer.ClusterGroup(group)

		// Generate patterns from closures
		patterns := o.generator.GeneratePatternsFromClosures(closures)
		allPatterns = append(allPatterns, patterns...)

		// Free MEMO table memory after each group
		o.clusterer.ClearMemo()
	}

	// Stage 4: Number range compression
	if o.config.EnableCompression {
		for _, pattern := range allPatterns {
			o.compressor.CompressPattern(pattern)
		}
	}

	// Stage 5: Quality filtering
	filteredPatterns := o.qualityFilter.FilterPatterns(allPatterns)

	// Stage 6: Deduplication
	if o.config.EnableDedupe {
		uniquePatterns := o.deduplicatePatterns(filteredPatterns)
		filteredPatterns = uniquePatterns
	}

	gologger.Info().Msgf("Pattern induction complete: %d patterns learned", len(filteredPatterns))

	return filteredPatterns, nil
}

// deduplicatePatterns removes duplicate patterns based on regex
func (o *Orchestrator) deduplicatePatterns(patterns []*Pattern) []*Pattern {
	seen := make(map[string]bool)
	unique := []*Pattern{}

	for _, pattern := range patterns {
		if !seen[pattern.Regex] {
			seen[pattern.Regex] = true
			unique = append(unique, pattern)
		}
	}

	return unique
}

// Stats returns statistics about the orchestrator's state
type OrchestratorStats struct {
	TotalGroups      int
	TotalClosures    int
	TotalPatterns    int
	FilteredPatterns int
	FinalPatterns    int
}

// GetStats returns current statistics (placeholder for now)
func (o *Orchestrator) GetStats() *OrchestratorStats {
	return &OrchestratorStats{}
}
