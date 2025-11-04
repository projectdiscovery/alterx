package inducer

import (
	"fmt"
	"sort"

	"github.com/projectdiscovery/gologger"
	"golang.org/x/net/publicsuffix"
)

// LevelGroup represents a group of domains with the same structural depth
// This naturally separates domains by their hierarchical level count
//
// Example:
//
//	Domains with 2 levels: ["scheduler.api.projectdiscovery.io", "webhook.dev.projectdiscovery.io"]
//	LevelCount: 2
//	Root: "projectdiscovery.io"
type LevelGroup struct {
	LevelCount int      // Number of subdomain levels (1, 2, 3, ...)
	Root       string   // Root domain (eTLD+1)
	Domains    []string // Full domains with this level count
}

// CountLevels counts the number of subdomain levels in a domain
// This excludes the root domain (eTLD+1) itself
//
// Algorithm:
//  1. Extract root domain using publicsuffix
//  2. Remove root domain from full domain
//  3. Count dots in remaining subdomain part
//  4. Level count = dots + 1 (if subdomain exists)
//
// Examples:
//
//	"api.projectdiscovery.io" → 1 level (subdomain: "api")
//	"scheduler.api.projectdiscovery.io" → 2 levels (subdomain: "scheduler.api")
//	"scheduler.v1.api.projectdiscovery.io" → 3 levels (subdomain: "scheduler.v1.api")
//	"projectdiscovery.io" → 0 levels (no subdomain)
//
// Returns:
//   - levelCount: Number of subdomain levels
//   - root: Root domain (eTLD+1)
//   - error: If domain parsing fails
func CountLevels(domain string) (levelCount int, root string, error error) {
	// Get root domain (eTLD+1)
	rootDomain, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return 0, "", fmt.Errorf("invalid domain %s: %w", domain, err)
	}

	// If domain IS the root, it has 0 subdomain levels
	if domain == rootDomain {
		return 0, rootDomain, nil
	}

	// Extract subdomain part (everything before root)
	// For "scheduler.api.projectdiscovery.io", subdomain = "scheduler.api"
	if len(domain) <= len(rootDomain)+1 {
		// Domain is shorter than root + dot - shouldn't happen
		return 0, rootDomain, fmt.Errorf("malformed domain %s", domain)
	}

	subdomainPart := domain[:len(domain)-len(rootDomain)-1] // Remove ".rootDomain"

	// Count dots in subdomain part
	dotCount := 0
	for _, ch := range subdomainPart {
		if ch == '.' {
			dotCount++
		}
	}

	// Level count = dots + 1
	// "api" → 0 dots → 1 level
	// "scheduler.api" → 1 dot → 2 levels
	// "scheduler.v1.api" → 2 dots → 3 levels
	levelCount = dotCount + 1

	return levelCount, rootDomain, nil
}

// GroupByLevelCount groups domains by their structural depth (level count)
// This separates domains into independent groups for pattern learning
//
// Key insight: Domains with different level counts should generate different patterns:
//   - 1-level domains: {{p0}}.{{root}}
//   - 2-level domains: {{p0}}.{{p1}}.{{root}}
//   - 3-level domains: {{p0}}.{{p1}}.{{p2}}.{{root}}
//
// Algorithm:
//  1. Count levels for each domain
//  2. Group domains with same level count
//  3. Store root domain for each group
//
// Examples:
//
//	Input: [
//	  "api.projectdiscovery.io",           // 1 level
//	  "cdn.projectdiscovery.io",           // 1 level
//	  "scheduler.api.projectdiscovery.io", // 2 levels
//	  "webhook.dev.projectdiscovery.io",   // 2 levels
//	]
//
//	Output: {
//	  1: LevelGroup{LevelCount: 1, Domains: ["api.projectdiscovery.io", "cdn.projectdiscovery.io"]},
//	  2: LevelGroup{LevelCount: 2, Domains: ["scheduler.api.projectdiscovery.io", "webhook.dev.projectdiscovery.io"]},
//	}
//
// Returns:
//   - map[levelCount]*LevelGroup
//   - error if any domain fails to parse
func GroupByLevelCount(domains []string) (map[int]*LevelGroup, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains provided")
	}

	groups := make(map[int]*LevelGroup)

	for _, domain := range domains {
		// Count levels for this domain
		levelCount, root, err := CountLevels(domain)
		if err != nil {
			gologger.Warning().Msgf("Skipping domain %s: %v", domain, err)
			continue
		}

		// Skip root-only domains (0 levels)
		if levelCount == 0 {
			gologger.Debug().Msgf("Skipping root domain: %s", domain)
			continue
		}

		// Group by level count
		if groups[levelCount] == nil {
			groups[levelCount] = &LevelGroup{
				LevelCount: levelCount,
				Root:       root,
				Domains:    []string{},
			}
		}

		groups[levelCount].Domains = append(groups[levelCount].Domains, domain)
	}

	return groups, nil
}

// GetSortedLevelGroups returns level groups sorted by level count (ascending)
// This ensures we process simpler patterns before complex ones
//
// Example order:
//  1. 1-level domains ({{p0}}.{{root}})
//  2. 2-level domains ({{p0}}.{{p1}}.{{root}})
//  3. 3-level domains ({{p0}}.{{p1}}.{{p2}}.{{root}})
func GetSortedLevelGroups(groups map[int]*LevelGroup) []*LevelGroup {
	sorted := make([]*LevelGroup, 0, len(groups))
	for _, group := range groups {
		sorted = append(sorted, group)
	}

	// Sort by level count ascending
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].LevelCount < sorted[j].LevelCount
	})

	return sorted
}

// PrintLevelGroupStats logs statistics about level groups
// Useful for debugging and understanding domain distribution
func PrintLevelGroupStats(groups map[int]*LevelGroup) {
	if len(groups) == 0 {
		gologger.Warning().Msg("No level groups found")
		return
	}

	gologger.Verbose().Msgf("Detected %d level groups:", len(groups))

	sorted := GetSortedLevelGroups(groups)
	for _, group := range sorted {
		gologger.Verbose().Msgf("  Level %d: %d domains (root: %s)",
			group.LevelCount, len(group.Domains), group.Root)
	}
}
