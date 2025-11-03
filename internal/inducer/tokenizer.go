package inducer

import (
	"fmt"
	"regexp"
	"strings"

	urlutil "github.com/projectdiscovery/utils/url"
	"golang.org/x/net/publicsuffix"
)

var (
	// numberRegex matches numeric sequences in tokens
	// Used to split tokens like "api01" → ["api", "01"]
	numberRegex = regexp.MustCompile(`([0-9]+)`)
)

// Tokenize parses a domain or subdomain string into a structured TokenizedDomain
// Following the regulator algorithm:
// 1. Extract subdomain part (remove root domain using publicsuffix, if full domain)
// 2. Split subdomain by dots → levels
// 3. For each level: tokenize by dashes and numbers
//
// Accepts two input formats:
// - Full domain: "api-dev-01.staging.example.com" → extracts "api-dev-01.staging"
// - Subdomain only: "api-dev-01.staging" → uses as-is
//
// Example:
//   Input: "api-dev-01.staging.example.com"
//   Output: TokenizedDomain with:
//     - Level 0: ["api", "-dev", "-01"]
//     - Level 1: ["staging"]
func Tokenize(domain string) (*TokenizedDomain, error) {
	// Handle wildcard subdomains first (before URL parsing)
	input := domain
	if strings.Contains(input, "*") {
		if strings.HasPrefix(input, "*.") {
			input = strings.TrimPrefix(input, "*.")
		} else {
			// Wildcard in middle (e.g., "prod.*.example.com") - invalid
			return nil, fmt.Errorf("invalid wildcard in domain %s", domain)
		}
	}

	// Try to parse as URL to get hostname (handles http://api.example.com)
	// If parsing fails or hostname is empty, treat the input as a plain hostname/subdomain
	var hostname string
	URL, err := urlutil.Parse(input)
	if err == nil && URL.Hostname() != "" {
		hostname = URL.Hostname()
	} else {
		// Input is not a valid URL or has no hostname (e.g., "api-dev" or "api.staging")
		// Treat it as a plain hostname/subdomain string
		hostname = input
	}

	// Try to extract subdomain and root domain
	subdomain, root, err := extractSubdomain(hostname)
	if err != nil {
		// If extraction fails, assume input is already a subdomain-only string
		// This happens when passing preprocessed subdomains (e.g., "api-dev-01")
		subdomain = hostname
		root = "" // No root domain
	} else if subdomain == "" {
		// extractSubdomain succeeded but returned empty subdomain
		// This means the hostname is just a root domain (e.g., "example.com")
		// Return empty tokenized domain
		return &TokenizedDomain{
			Original:  domain,
			Subdomain: "",
			Root:      root,
			Levels:    []Level{},
		}, nil
	}

	// Split subdomain into levels (by dots)
	levelStrings := splitIntoLevels(subdomain)

	// Tokenize each level
	levels := make([]Level, 0, len(levelStrings))
	for levelIndex, levelStr := range levelStrings {
		tokens := tokenizeLevel(levelStr)
		levels = append(levels, Level{
			Index:  levelIndex,
			Tokens: tokens,
		})
	}

	return &TokenizedDomain{
		Original:  domain,
		Subdomain: subdomain,
		Root:      root,
		Levels:    levels,
	}, nil
}

// extractSubdomain separates the subdomain from the root domain
// Uses golang.org/x/net/publicsuffix for accurate eTLD detection
//
// Examples:
//   "api.example.com" → subdomain="api", root="example.com"
//   "api.staging.example.com" → subdomain="api.staging", root="example.com"
//   "example.com" → subdomain="", root="example.com"
func extractSubdomain(hostname string) (subdomain string, root string, error error) {
	// Get the root domain (eTLD+1) using publicsuffix
	rootDomain, err := publicsuffix.EffectiveTLDPlusOne(hostname)
	if err != nil {
		// This happens if the input is just a TLD (e.g., ".com" or "co.uk")
		return "", hostname, fmt.Errorf("hostname %s is not a valid domain", hostname)
	}

	// Everything before the root domain is the subdomain
	if hostname == rootDomain {
		// No subdomain, just the root
		return "", rootDomain, nil
	}

	subdomainPrefix := strings.TrimSuffix(hostname, "."+rootDomain)
	return subdomainPrefix, rootDomain, nil
}

// splitIntoLevels splits a subdomain by dots into DNS hierarchy levels
// Levels are ordered left-to-right (leftmost = level 0)
//
// Examples:
//   "api" → ["api"]
//   "api.staging" → ["api", "staging"]
//   "api.v1.staging" → ["api", "v1", "staging"]
func splitIntoLevels(subdomain string) []string {
	if subdomain == "" {
		return []string{}
	}
	return strings.Split(subdomain, ".")
}

// tokenizeLevel breaks a single DNS level into structured tokens
// Following the regulator algorithm:
// 1. Split by dashes: "api-dev-01" → ["api", "dev", "01"]
// 2. Prefix all but first with "-": ["api", "-dev", "-01"]
// 3. Split each token by numbers: "api01" → ["api", "01"]
// 4. Preserve hyphenated numbers: "-01" stays as "-01"
//
// Examples:
//   "api" → [Token{Value:"api", Type:Word}]
//   "api-dev" → [Token{Value:"api", Type:Word}, Token{Value:"-dev", Type:Dash}]
//   "api-dev-01" → [Token{Value:"api", Type:Word}, Token{Value:"-dev", Type:Dash}, Token{Value:"-01", Type:Dash}]
//   "api01" → [Token{Value:"api", Type:Word}, Token{Value:"01", Type:Number}]
func tokenizeLevel(level string) []Token {
	if level == "" {
		return []Token{}
	}

	tokens := []Token{}
	position := 0

	// Step 1: Split by dashes
	parts := strings.Split(level, "-")

	for i, part := range parts {
		if part == "" {
			// Skip empty parts (e.g., from leading/trailing dashes)
			continue
		}

		// Step 2: Prefix all but first with "-"
		if i > 0 {
			// This is a dash-separated part
			// Check if it's purely numeric (hyphenated number like "-01")
			if isNumeric(part) {
				// Hyphenated number: keep as single token "-01"
				tokens = append(tokens, Token{
					Value:    "-" + part,
					Type:     TokenTypeDash,
					Position: position,
				})
				position++
			} else {
				// Mixed content: split by numbers
				subTokens := splitByNumbers("-" + part)
				for _, subToken := range subTokens {
					tokens = append(tokens, Token{
						Value:    subToken,
						Type:     classifyToken(subToken),
						Position: position,
					})
					position++
				}
			}
		} else {
			// First part: no dash prefix
			// Split by numbers
			subTokens := splitByNumbers(part)
			for _, subToken := range subTokens {
				tokens = append(tokens, Token{
					Value:    subToken,
					Type:     classifyToken(subToken),
					Position: position,
				})
				position++
			}
		}
	}

	return tokens
}

// splitByNumbers splits a token by numeric sequences
// Preserves the numbers as separate tokens
//
// Examples:
//   "api" → ["api"]
//   "api01" → ["api", "01"]
//   "server123test" → ["server", "123", "test"]
//   "01" → ["01"]
func splitByNumbers(token string) []string {
	if token == "" {
		return []string{}
	}

	// Special case: if token starts with dash and rest is numeric, don't split
	if strings.HasPrefix(token, "-") && isNumeric(token[1:]) {
		return []string{token}
	}

	// Use regex to split by number sequences, but keep the numbers
	parts := numberRegex.Split(token, -1)
	numbers := numberRegex.FindAllString(token, -1)

	// Interleave parts and numbers
	result := []string{}
	for i, part := range parts {
		if part != "" {
			result = append(result, part)
		}
		if i < len(numbers) {
			result = append(result, numbers[i])
		}
	}

	return result
}

// classifyToken determines the type of a token
func classifyToken(token string) TokenType {
	if token == "" {
		return TokenTypeWord
	}

	// Dash-prefixed tokens
	if strings.HasPrefix(token, "-") {
		return TokenTypeDash
	}

	// Pure numeric tokens
	if isNumeric(token) {
		return TokenTypeNumber
	}

	// Default: word token
	return TokenTypeWord
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
