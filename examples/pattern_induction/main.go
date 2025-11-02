package main

import (
	"fmt"
	"os"

	"github.com/projectdiscovery/alterx"
)

// Example demonstrating pattern induction feature
// This shows how to use the "inferred" mode to learn patterns from existing subdomains
func main() {
	// Simulate passive subdomain enumeration results
	// In real usage, these would come from tools like subfinder, amass, chaos, etc.
	passiveSubdomains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-dev-03.example.com",
		"api-prod-01.example.com",
		"api-prod-02.example.com",
		"api-prod-03.example.com",
		"web-staging.example.com",
		"web-prod.example.com",
		"cdn-us-east.example.com",
		"cdn-eu-west.example.com",
		"db-primary-01.example.com",
		"db-replica-01.example.com",
		"db-replica-02.example.com",
	}

	fmt.Println("=== Pattern Induction Example ===")
	fmt.Printf("Input: %d passive subdomains\n\n", len(passiveSubdomains))

	// Create AlterX options with both mode (learned + default patterns)
	opts := &alterx.Options{
		Domains: passiveSubdomains,
		Mode:    "both", // Use both learned and default patterns
		Limit:   50,     // Limit output for demonstration
	}

	// Create mutator instance
	m, err := alterx.New(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Execute to generate permutations based on learned patterns
	fmt.Println("Learned patterns will be applied to generate new permutations...")
	fmt.Println("\nSample generated subdomains:")
	fmt.Println("---")

	// Execute and print results
	if err := m.ExecuteWithWriter(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("---")
	fmt.Println("\nUsage modes:")
	fmt.Println("  - 'default': Use only predefined patterns from permutations.yaml")
	fmt.Println("  - 'inferred': Use only patterns learned from passive enumeration")
	fmt.Println("  - 'both': Combine predefined and learned patterns (recommended)")
}
