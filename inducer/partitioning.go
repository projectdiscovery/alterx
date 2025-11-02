package inducer

import (
	"sort"

	"github.com/projectdiscovery/gologger"
)

// DomainGroup represents a bounded group of similar domains
// This is the key to O(1) memory - each group is processed independently
type DomainGroup struct {
	Prefix  string   // The prefix that defines this group (e.g., "api", "api-dev")
	Domains []string // The actual domain strings (bounded to maxGroupSize)
	Size    int      // Number of domains in this group
}

// Partitioner implements hierarchical prefix partitioning
// This splits N domains into k groups where each group has ≤ maxGroupSize domains
// Following the proposed optimization strategy from literature_survey/proposed_solution.md
type Partitioner struct {
	maxGroupSize int  // Maximum domains per group (default: 5000)
	trie         *Trie // Prefix trie for efficient grouping
}

// NewPartitioner creates a new domain partitioner
func NewPartitioner(maxGroupSize int) *Partitioner {
	if maxGroupSize <= 0 {
		maxGroupSize = 5000 // Default from optimization strategy
	}

	return &Partitioner{
		maxGroupSize: maxGroupSize,
		trie:         NewTrie(),
	}
}

// Partition splits domains into bounded groups using hierarchical prefix partitioning
// Algorithm:
// 1. Build trie from all domains
// 2. Try 1-gram prefixes (a, b, c, ..., 0, 1, 2, ...)
// 3. If a group is too large, split by 2-gram (aa, ab, ac, ...)
// 4. Continue recursively up to 4-grams
// 5. If still too large, apply random sampling
//
// Returns list of groups, each with ≤ maxGroupSize domains
func (p *Partitioner) Partition(domains []string) []*DomainGroup {
	if len(domains) == 0 {
		return []*DomainGroup{}
	}

	// Build trie
	for _, domain := range domains {
		p.trie.Insert(domain)
	}

	// Start with 1-gram prefixes
	groups := []*DomainGroup{}
	prefixes := p.generate1Grams()

	for _, prefix := range prefixes {
		matches := p.trie.KeysWithPrefix(prefix)
		if len(matches) == 0 {
			continue
		}

		if len(matches) <= p.maxGroupSize {
			// Group is small enough, add it
			groups = append(groups, &DomainGroup{
				Prefix:  prefix,
				Domains: matches,
				Size:    len(matches),
			})
		} else {
			// Group too large, sub-partition
			subgroups := p.subPartition(prefix, matches, 2)
			groups = append(groups, subgroups...)
		}
	}

	// Handle domains that don't match any prefix (edge case)
	// This shouldn't happen in practice but handles it gracefully
	if len(groups) == 0 && len(domains) > 0 {
		gologger.Warning().Msgf("No prefix matches found, creating single group")
		groups = append(groups, &DomainGroup{
			Prefix:  "",
			Domains: domains,
			Size:    len(domains),
		})
	}

	// Log group size distribution
	p.logGroupStats(groups)

	return groups
}

// subPartition recursively partitions a group using longer prefixes
// ngramLen: current n-gram length (2, 3, or 4)
func (p *Partitioner) subPartition(basePrefix string, domains []string, ngramLen int) []*DomainGroup {
	if len(domains) <= p.maxGroupSize {
		// Small enough, return as single group
		return []*DomainGroup{{
			Prefix:  basePrefix,
			Domains: domains,
			Size:    len(domains),
		}}
	}

	if ngramLen > 4 {
		// Reached max depth, apply sampling if still too large
		return p.sampleLargeGroup(basePrefix, domains)
	}

	// Generate n-gram extensions
	groups := []*DomainGroup{}
	ngrams := p.generateNGrams(ngramLen)

	// Try each n-gram extension
	for _, ngram := range ngrams {
		extendedPrefix := basePrefix + ngram
		matches := []string{}

		// Filter domains that match extended prefix
		for _, domain := range domains {
			if len(domain) >= len(extendedPrefix) && domain[:len(extendedPrefix)] == extendedPrefix {
				matches = append(matches, domain)
			}
		}

		if len(matches) == 0 {
			continue
		}

		if len(matches) <= p.maxGroupSize {
			// Group is small enough
			groups = append(groups, &DomainGroup{
				Prefix:  extendedPrefix,
				Domains: matches,
				Size:    len(matches),
			})
		} else {
			// Still too large, recurse deeper
			subgroups := p.subPartition(extendedPrefix, matches, ngramLen+1)
			groups = append(groups, subgroups...)
		}
	}

	// If no groups were created, fall back to sampling
	if len(groups) == 0 {
		return p.sampleLargeGroup(basePrefix, domains)
	}

	return groups
}

// sampleLargeGroup handles groups that are still too large after max recursion depth
// Strategy: Split into multiple groups of maxGroupSize using sequential sampling
func (p *Partitioner) sampleLargeGroup(prefix string, domains []string) []*DomainGroup {
	gologger.Warning().Msgf("Group %s still has %d domains after 4-gram, splitting sequentially", prefix, len(domains))

	groups := []*DomainGroup{}
	for i := 0; i < len(domains); i += p.maxGroupSize {
		end := i + p.maxGroupSize
		if end > len(domains) {
			end = len(domains)
		}

		groups = append(groups, &DomainGroup{
			Prefix:  prefix,
			Domains: domains[i:end],
			Size:    end - i,
		})
	}

	return groups
}

// generate1Grams generates single-character prefixes
func (p *Partitioner) generate1Grams() []string {
	prefixes := make([]string, 0, 36)

	// a-z
	for c := 'a'; c <= 'z'; c++ {
		prefixes = append(prefixes, string(c))
	}

	// 0-9
	for c := '0'; c <= '9'; c++ {
		prefixes = append(prefixes, string(c))
	}

	return prefixes
}

// generateNGrams generates n-character prefixes
func (p *Partitioner) generateNGrams(n int) []string {
	if n < 1 {
		return []string{}
	}

	chars := "abcdefghijklmnopqrstuvwxyz0123456789-"

	// For n=2: generate all 2-char combinations
	// For n=3: generate all 3-char combinations
	// etc.

	var generate func(current string, depth int) []string
	generate = func(current string, depth int) []string {
		if depth == 0 {
			return []string{current}
		}

		results := []string{}
		for _, c := range chars {
			extended := current + string(c)
			results = append(results, generate(extended, depth-1)...)
		}
		return results
	}

	return generate("", n)
}

// logGroupStats logs statistics about group sizes
func (p *Partitioner) logGroupStats(groups []*DomainGroup) {
	if len(groups) == 0 {
		return
	}

	// Calculate statistics
	sizes := make([]int, len(groups))
	for i, group := range groups {
		sizes[i] = group.Size
	}

	sort.Ints(sizes)

	// Check for oversized groups
	oversized := 0
	for _, size := range sizes {
		if size > p.maxGroupSize {
			oversized++
		}
	}

	if oversized > 0 {
		gologger.Warning().Msgf("%d groups exceed max size %d", oversized, p.maxGroupSize)
	}
}

// Trie implements a prefix tree for efficient string matching
// This is used for fast prefix-based domain grouping
type Trie struct {
	root *TrieNode
}

// TrieNode represents a node in the prefix tree
type TrieNode struct {
	children map[rune]*TrieNode // Child nodes indexed by character
	isEnd    bool               // True if this node represents end of a string
	value    string             // The complete string (stored only at terminal nodes)
}

// NewTrie creates a new empty trie
func NewTrie() *Trie {
	return &Trie{
		root: &TrieNode{
			children: make(map[rune]*TrieNode),
		},
	}
}

// Insert adds a string to the trie
func (t *Trie) Insert(s string) {
	node := t.root
	for _, ch := range s {
		if node.children[ch] == nil {
			node.children[ch] = &TrieNode{
				children: make(map[rune]*TrieNode),
			}
		}
		node = node.children[ch]
	}
	node.isEnd = true
	node.value = s
}

// KeysWithPrefix returns all strings in the trie that start with the given prefix
func (t *Trie) KeysWithPrefix(prefix string) []string {
	// Navigate to the prefix node
	node := t.root
	for _, ch := range prefix {
		if node.children[ch] == nil {
			return []string{} // Prefix not found
		}
		node = node.children[ch]
	}

	// Collect all strings under this node
	results := []string{}
	t.collectKeys(node, &results)
	return results
}

// collectKeys recursively collects all strings under a node
func (t *Trie) collectKeys(node *TrieNode, results *[]string) {
	if node.isEnd {
		*results = append(*results, node.value)
	}

	for _, child := range node.children {
		t.collectKeys(child, results)
	}
}

// Size returns the total number of strings in the trie
func (t *Trie) Size() int {
	count := 0
	t.countNodes(t.root, &count)
	return count
}

// countNodes recursively counts terminal nodes
func (t *Trie) countNodes(node *TrieNode, count *int) {
	if node.isEnd {
		*count++
	}

	for _, child := range node.children {
		t.countNodes(child, count)
	}
}
