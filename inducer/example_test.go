package inducer_test

import (
	"fmt"

	"github.com/projectdiscovery/alterx/inducer"
)

// Example demonstrates basic usage of the pattern inducer
func Example() {
	// Sample domains (typically from Options.Domains)
	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
		"web-staging.example.com",
		"web-prod.example.com",
		"db01.internal.example.com",
		"db02.internal.example.com",
	}

	// Create pattern inducer with default config
	pi := inducer.NewPatternInducer(domains, nil)

	// Load and tokenize all domains
	err := pi.LoadAndTokenize()
	if err != nil {
		panic(err)
	}

	// Get statistics
	stats := pi.Stats()
	fmt.Printf("Successfully tokenized %d domains\n", stats.TokenizedDomains)
	fmt.Printf("Found %d levels\n", len(pi.GetAllLevelStats()))

	// Access tokenized domains
	for i := 0; i < stats.TokenizedDomains; i++ {
		td := pi.GetTokenizedDomain(i)
		fmt.Printf("\nDomain: %s\n", td.Original)
		fmt.Printf("  Subdomain: %s\n", td.Subdomain)
		fmt.Printf("  Root: %s\n", td.Root)
		fmt.Printf("  Levels: %d\n", td.GetLevelCount())

		// Show tokens at each level
		for levelIdx, level := range td.Levels {
			fmt.Printf("  Level %d (index %d):", levelIdx+1, levelIdx)
			for _, token := range level.Tokens {
				fmt.Printf(" [%s]", token.Value)
			}
			fmt.Println()
		}
	}

	// Output:
	// Successfully tokenized 7 domains
	// Found 2 levels
	//
	// Domain: api-dev-01.example.com
	//   Subdomain: api-dev-01
	//   Root: example.com
	//   Levels: 1
	//   Level 1 (index 0): [api] [-dev] [-01]
	//
	// Domain: api-dev-02.example.com
	//   Subdomain: api-dev-02
	//   Root: example.com
	//   Levels: 1
	//   Level 1 (index 0): [api] [-dev] [-02]
	//
	// Domain: api-prod-01.example.com
	//   Subdomain: api-prod-01
	//   Root: example.com
	//   Levels: 1
	//   Level 1 (index 0): [api] [-prod] [-01]
	//
	// Domain: web-staging.example.com
	//   Subdomain: web-staging
	//   Root: example.com
	//   Levels: 1
	//   Level 1 (index 0): [web] [-staging]
	//
	// Domain: web-prod.example.com
	//   Subdomain: web-prod
	//   Root: example.com
	//   Levels: 1
	//   Level 1 (index 0): [web] [-prod]
	//
	// Domain: db01.internal.example.com
	//   Subdomain: db01.internal
	//   Root: example.com
	//   Levels: 2
	//   Level 1 (index 0): [db] [01]
	//   Level 2 (index 1): [internal]
	//
	// Domain: db02.internal.example.com
	//   Subdomain: db02.internal
	//   Root: example.com
	//   Levels: 2
	//   Level 1 (index 0): [db] [02]
	//   Level 2 (index 1): [internal]
}

// ExamplePatternInducer_GetLevel demonstrates accessing specific levels
func ExamplePatternInducer_GetLevel() {
	domains := []string{
		"api-dev.staging.example.com",
		"web-prod.staging.example.com",
		"cdn.prod.example.com",
	}

	pi := inducer.NewPatternInducer(domains, nil)
	if err := pi.LoadAndTokenize(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Get all level 1 data (leftmost subdomain level - 0-indexed internally)
	level1Data := pi.GetLevel(1)
	fmt.Printf("Level 1 has %d entries\n", len(level1Data))

	// Get all level 2 data (second subdomain level)
	level2Data := pi.GetLevel(2)
	fmt.Printf("Level 2 has %d entries\n", len(level2Data))

	// Output:
	// Level 1 has 3 entries
	// Level 2 has 3 entries
}

// ExamplePatternInducer_GetLevelStats demonstrates statistics access
func ExamplePatternInducer_GetLevelStats() {
	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
		"web-staging.example.com",
	}

	pi := inducer.NewPatternInducer(domains, nil)
	if err := pi.LoadAndTokenize(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Get statistics for level 1 (leftmost subdomain level)
	stats := pi.GetLevelStats(1)
	if stats != nil {
		fmt.Printf("Level 1 statistics:\n")
		fmt.Printf("  Domains at this level: %d\n", stats.DomainCount)
		fmt.Printf("  Max position: %d\n", stats.MaxPosition)
		fmt.Printf("  Unique tokens: %d\n", len(stats.TokenCounts))

		// Get top tokens count
		fmt.Printf("  Top token count: %d\n", len(stats.GetTopTokens(3)))
	}

	// Output:
	// Level 1 statistics:
	//   Domains at this level: 4
	//   Max position: 2
	//   Unique tokens: 7
	//   Top token count: 3
}
