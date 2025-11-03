package inducer

import (
	"sort"
	"strings"
)

// TrieNode represents a node in the prefix tree
// Each node stores children keyed by rune and domain IDs at terminal nodes
type TrieNode struct {
	Children  map[rune]*TrieNode // Child nodes keyed by character
	IsEnd     bool               // True if this node represents a complete domain
	DomainIDs []int              // Indices of domains ending at this node
}

// Trie implements a prefix tree for fast domain prefix lookups
// This is used for Strategy 2 (N-gram prefix anchoring) to efficiently
// group domains by common prefixes without O(N²) comparisons
type Trie struct {
	Root    *TrieNode // Root node of the trie
	Domains []string  // Original domain list (for reference by ID)
}

// NewTrie creates a new Trie from a list of domains
// Each domain is inserted with its index as the domain ID
func NewTrie(domains []string) *Trie {
	trie := &Trie{
		Root:    newTrieNode(),
		Domains: make([]string, len(domains)),
	}

	// Copy domains and insert each into trie
	copy(trie.Domains, domains)
	for i, domain := range domains {
		trie.Insert(domain, i)
	}

	return trie
}

// newTrieNode creates a new empty trie node
func newTrieNode() *TrieNode {
	return &TrieNode{
		Children:  make(map[rune]*TrieNode),
		IsEnd:     false,
		DomainIDs: []int{},
	}
}

// Insert adds a domain to the trie with the given domain ID
// The domain is indexed character-by-character for prefix matching
func (t *Trie) Insert(domain string, domainID int) {
	node := t.Root

	// Traverse or create path for each character
	for _, ch := range domain {
		if node.Children[ch] == nil {
			node.Children[ch] = newTrieNode()
		}
		node = node.Children[ch]
	}

	// Mark end of domain and store ID
	node.IsEnd = true
	node.DomainIDs = append(node.DomainIDs, domainID)
}

// SearchPrefix finds all domain IDs that start with the given prefix
// Returns slice of domain IDs that have this prefix
// Returns empty slice if no domains match
func (t *Trie) SearchPrefix(prefix string) []int {
	node := t.Root

	// Navigate to prefix node
	for _, ch := range prefix {
		if node.Children[ch] == nil {
			// Prefix doesn't exist in trie
			return []int{}
		}
		node = node.Children[ch]
	}

	// Collect all domain IDs in subtree rooted at prefix node
	return t.collectDomainIDs(node)
}

// collectDomainIDs performs DFS to collect all domain IDs in subtree
func (t *Trie) collectDomainIDs(node *TrieNode) []int {
	if node == nil {
		return []int{}
	}

	domainIDs := []int{}

	// Add IDs at this node
	if node.IsEnd {
		domainIDs = append(domainIDs, node.DomainIDs...)
	}

	// Recursively collect from children
	for _, child := range node.Children {
		childIDs := t.collectDomainIDs(child)
		domainIDs = append(domainIDs, childIDs...)
	}

	return domainIDs
}

// GetNgramPrefixes extracts all N-gram prefixes from domains and groups them
// Returns map[prefix][]domainIDs
//
// For n=1: Groups by first character ("a" → ["api.example.com", "admin.example.com"])
// For n=2: Groups by first 2 chars ("ap" → ["api.example.com"], "ad" → ["admin.example.com"])
// For n=3: Groups by first 3 chars, etc.
//
// This is the core operation for Strategy 2 (N-gram prefix anchoring)
func (t *Trie) GetNgramPrefixes(n int) map[string][]int {
	if n <= 0 {
		return make(map[string][]int)
	}

	prefixGroups := make(map[string][]int)

	// Extract N-gram prefix from each domain
	for i, domain := range t.Domains {
		prefix := t.extractNgramPrefix(domain, n)
		if prefix != "" {
			prefixGroups[prefix] = append(prefixGroups[prefix], i)
		}
	}

	return prefixGroups
}

// extractNgramPrefix extracts the first N characters from a domain
// Returns empty string if domain is shorter than N characters
func (t *Trie) extractNgramPrefix(domain string, n int) string {
	runes := []rune(domain)
	if len(runes) < n {
		// Domain too short - return whole domain as prefix
		// This groups short domains together
		return domain
	}
	return string(runes[:n])
}

// GetPrefixGroups returns all domains grouped by their prefixes of length n
// This is an alternative interface that returns actual domains instead of IDs
func (t *Trie) GetPrefixGroups(n int) map[string][]string {
	prefixIDs := t.GetNgramPrefixes(n)
	prefixGroups := make(map[string][]string, len(prefixIDs))

	for prefix, domainIDs := range prefixIDs {
		domains := make([]string, len(domainIDs))
		for i, id := range domainIDs {
			domains[i] = t.Domains[id]
		}
		prefixGroups[prefix] = domains
	}

	return prefixGroups
}

// GetTokenGroups groups domains by their first token value
// This is used for Strategy 3 (Token-level clustering)
//
// Returns map[firstToken][]domainIDs
// Example: "api-dev.example.com" → "api" group
func (t *Trie) GetTokenGroups() map[string][]int {
	tokenGroups := make(map[string][]int)

	for i, domain := range t.Domains {
		// Tokenize domain to extract first token
		td, err := Tokenize(domain)
		if err != nil || len(td.Levels) == 0 || len(td.Levels[0].Tokens) == 0 {
			// Failed to tokenize or no tokens - skip
			continue
		}

		// Get first token value
		firstToken := td.Levels[0].Tokens[0].Value
		tokenGroups[firstToken] = append(tokenGroups[firstToken], i)
	}

	return tokenGroups
}

// GetTokenGroupDomains returns token groups as domain strings instead of IDs
func (t *Trie) GetTokenGroupDomains() map[string][]string {
	tokenIDs := t.GetTokenGroups()
	tokenGroups := make(map[string][]string, len(tokenIDs))

	for token, domainIDs := range tokenIDs {
		domains := make([]string, len(domainIDs))
		for i, id := range domainIDs {
			domains[i] = t.Domains[id]
		}
		tokenGroups[token] = domains
	}

	return tokenGroups
}

// Stats returns statistics about the trie structure
type TrieStats struct {
	TotalDomains int            // Number of domains in trie
	TotalNodes   int            // Number of trie nodes
	MaxDepth     int            // Maximum depth of trie
	PrefixGroups map[int]int    // N-gram size → number of groups
}

// GetStats returns statistics about the trie
func (t *Trie) GetStats() *TrieStats {
	stats := &TrieStats{
		TotalDomains: len(t.Domains),
		PrefixGroups: make(map[int]int),
	}

	// Count nodes and depth
	stats.TotalNodes, stats.MaxDepth = t.countNodesAndDepth(t.Root, 0)

	// Count prefix groups for different N-gram sizes
	for n := 1; n <= 3; n++ {
		groups := t.GetNgramPrefixes(n)
		stats.PrefixGroups[n] = len(groups)
	}

	return stats
}

// countNodesAndDepth recursively counts nodes and finds max depth
func (t *Trie) countNodesAndDepth(node *TrieNode, depth int) (nodeCount int, maxDepth int) {
	if node == nil {
		return 0, depth
	}

	nodeCount = 1 // Count this node
	maxDepth = depth

	// Recurse on children
	for _, child := range node.Children {
		childNodes, childDepth := t.countNodesAndDepth(child, depth+1)
		nodeCount += childNodes
		if childDepth > maxDepth {
			maxDepth = childDepth
		}
	}

	return nodeCount, maxDepth
}

// GetLongestCommonPrefix finds the longest common prefix among all domains
// Returns the LCP string
func (t *Trie) GetLongestCommonPrefix() string {
	if len(t.Domains) == 0 {
		return ""
	}

	if len(t.Domains) == 1 {
		return t.Domains[0]
	}

	// Start from root and walk down as long as there's only one child
	var prefix strings.Builder
	node := t.Root

	for len(node.Children) == 1 && !node.IsEnd {
		// Single child - add to prefix
		for ch, child := range node.Children {
			prefix.WriteRune(ch)
			node = child
			break
		}
	}

	return prefix.String()
}

// GetAllPrefixes returns all prefixes in the trie (including intermediate ones)
// This can be useful for analysis and debugging
func (t *Trie) GetAllPrefixes() []string {
	prefixes := []string{}
	t.collectPrefixes(t.Root, "", &prefixes)
	sort.Strings(prefixes)
	return prefixes
}

// collectPrefixes performs DFS to collect all prefixes
func (t *Trie) collectPrefixes(node *TrieNode, prefix string, prefixes *[]string) {
	if node == nil {
		return
	}

	// Add current prefix if it's a complete domain
	if node.IsEnd {
		*prefixes = append(*prefixes, prefix)
	}

	// Recurse on children
	for ch, child := range node.Children {
		t.collectPrefixes(child, prefix+string(ch), prefixes)
	}
}
