package inducer

// Closure represents a group of similar domains based on edit distance
// This is the core data structure for pattern generation
type Closure struct {
	Domains []string // The domains in this closure
	Delta   int      // The edit distance threshold used
	Size    int      // Number of domains
}

// ClusteringConfig controls the clustering behavior
type ClusteringConfig struct {
	MinDelta int // Minimum delta to try (default: 2)
	MaxDelta int // Maximum delta to try (default: 10)
}

// DefaultClusteringConfig returns sensible defaults
func DefaultClusteringConfig() *ClusteringConfig {
	return &ClusteringConfig{
		MinDelta: 2,
		MaxDelta: 10,
	}
}

// Clusterer implements multi-strategy edit distance clustering
// Following regulator's three-strategy approach
type Clusterer struct {
	config *ClusteringConfig
	memo   *EditDistanceMemo
}

// NewClusterer creates a new clusterer with optional config
func NewClusterer(config *ClusteringConfig) *Clusterer {
	if config == nil {
		config = DefaultClusteringConfig()
	}

	return &Clusterer{
		config: config,
		memo:   NewEditDistanceMemo(),
	}
}

// ClusterGroup applies all clustering strategies to a domain group
// This is the main entry point that orchestrates the three strategies
func (c *Clusterer) ClusterGroup(group *DomainGroup) []*Closure {
	if len(group.Domains) == 0 {
		return []*Closure{}
	}

	// Build MEMO table for this group
	// This is bounded memory: O(N²) where N ≤ maxGroupSize
	c.memo.Clear() // Clear previous group's data
	c.memo.PrecomputeDistances(group.Domains)

	allClosures := []*Closure{}

	// Strategy 1: Global edit distance clustering
	globalClosures := c.strategyGlobal(group.Domains)
	allClosures = append(allClosures, globalClosures...)

	// Strategy 2: N-gram prefix clustering (simplified for Go implementation)
	// TODO: Implement trie-based n-gram prefix strategy if needed
	// For now, global + token strategies provide good coverage

	// Strategy 3: Token-level clustering
	tokenClosures := c.strategyTokenLevel(group.Domains)
	allClosures = append(allClosures, tokenClosures...)

	// Deduplicate closures
	uniqueClosures := c.deduplicateClosures(allClosures)

	return uniqueClosures
}

// strategyGlobal implements Strategy 1: Global edit distance clustering
// Try multiple delta values (2 through 10) and find closures at each level
func (c *Clusterer) strategyGlobal(domains []string) []*Closure {
	closures := []*Closure{}

	for delta := c.config.MinDelta; delta <= c.config.MaxDelta; delta++ {
		deltaClosures := c.editClosures(domains, delta)
		closures = append(closures, deltaClosures...)
	}

	return closures
}

// strategyTokenLevel implements Strategy 3: Token-level prefix + edit distance
// Group by first token, then apply edit distance within each group
func (c *Clusterer) strategyTokenLevel(domains []string) []*Closure {
	// Extract first tokens
	tokenGroups := make(map[string][]string)

	for _, domain := range domains {
		// Extract first token (simplified - just take chars before first special char)
		firstToken := extractFirstToken(domain)
		tokenGroups[firstToken] = append(tokenGroups[firstToken], domain)
	}

	closures := []*Closure{}

	// Apply edit distance clustering within each token group
	for _, group := range tokenGroups {
		if len(group) < 2 {
			continue // Need at least 2 domains for a pattern
		}

		// Try different delta values for this token group
		for delta := c.config.MinDelta; delta <= c.config.MaxDelta; delta++ {
			groupClosures := c.editClosures(group, delta)
			closures = append(closures, groupClosures...)
		}
	}

	return closures
}

// editClosures finds all edit distance closures at a given delta
// This is the core clustering algorithm from regulator
func (c *Clusterer) editClosures(domains []string, delta int) []*Closure {
	if len(domains) < 2 {
		return []*Closure{}
	}

	closures := []*Closure{}
	processed := make(map[string]bool)

	// For each domain, find its neighbors within delta distance
	for _, domain := range domains {
		if processed[domain] {
			continue
		}

		neighbors := []string{domain}
		processed[domain] = true

		// Find all domains within delta distance
		for _, other := range domains {
			if other == domain {
				continue
			}

			dist := c.memo.Distance(domain, other)
			if dist <= delta {
				neighbors = append(neighbors, other)
				processed[other] = true
			}
		}

		// Only keep closures with multiple domains
		if len(neighbors) > 1 {
			closures = append(closures, &Closure{
				Domains: neighbors,
				Delta:   delta,
				Size:    len(neighbors),
			})
		}
	}

	return closures
}

// deduplicateClosures removes duplicate closures
// Two closures are duplicates if they contain the exact same set of domains
func (c *Clusterer) deduplicateClosures(closures []*Closure) []*Closure {
	if len(closures) == 0 {
		return closures
	}

	// Create unique set using string key
	seen := make(map[string]bool)
	unique := []*Closure{}

	for _, closure := range closures {
		key := makeClosureKey(closure)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, closure)
		}
	}

	return unique
}

// makeClosureKey creates a unique string key for a closure
// Used for deduplication
func makeClosureKey(closure *Closure) string {
	// Sort domains for consistent key
	sorted := make([]string, len(closure.Domains))
	copy(sorted, closure.Domains)

	// Simple sorting (could use sort.Strings but this is fast enough)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Concatenate to form key
	key := ""
	for _, domain := range sorted {
		key += domain + "|"
	}

	return key
}

// extractFirstToken extracts the first token from a domain
// This is a simplified version - full implementation would use the tokenizer
func extractFirstToken(domain string) string {
	// Find first special character (., -, or digit transition)
	for i, ch := range domain {
		if ch == '.' || ch == '-' {
			return domain[:i]
		}
	}
	return domain
}

// GetMemo returns the internal MEMO table (for debugging/stats)
func (c *Clusterer) GetMemo() *EditDistanceMemo {
	return c.memo
}

// ClearMemo clears the internal MEMO table to free memory
func (c *Clusterer) ClearMemo() {
	c.memo.Clear()
}
