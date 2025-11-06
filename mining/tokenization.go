package mining

import (
	"regexp"
	"strings"
)

// TokenizedSubdomain represents a tokenized subdomain with hierarchical structure.
type TokenizedSubdomain struct {
	// Original is the original subdomain string that was tokenized
	Original string
	// Levels contains the tokenized levels of the subdomain hierarchy
	Levels []Level
}

// Level represents a single level in the subdomain hierarchy with its tokens.
// For example, in "api-prod-12.dev", there are two levels:
//   - Level 0: {Label: "api-prod-12", Tokens: ["api", "-prod", "-12"]}
//   - Level 1: {Label: "dev", Tokens: ["dev"]}
type Level struct {
	// Label is the original label at this level (e.g., "api-prod-12")
	Label string
	// Tokens are the individual tokens extracted from the label
	Tokens []string
}

// Tokenize converts subdomains into structured tokenized representations.
// It splits subdomains by dots into hierarchical levels and tokenizes each level
// by hyphens and numbers while preserving hyphen prefixes.
//
// NOTE: Input should be subdomain parts only (root domain already removed).
// For example: "api-prod-12" or "api.dev" (not "api.dev.example.com")
//
// EXAMPLE:
//
//	Input:  ["api-prod-12", "web", "api5.dev-staging2"]
//	Output: []TokenizedSubdomain{
//	  {
//	    Original: "api-prod-12",
//	    Levels: []Level{
//	      {Label: "api-prod-12", Tokens: []string{"api", "-prod", "-12"}},
//	    },
//	  },
//	  {
//	    Original: "web",
//	    Levels: []Level{
//	      {Label: "web", Tokens: []string{"web"}},
//	    },
//	  },
//	  {
//	    Original: "api5.dev-staging2",
//	    Levels: []Level{
//	      {Label: "api5", Tokens: []string{"api", "5"}},
//	      {Label: "dev-staging2", Tokens: []string{"dev", "-staging", "2"}},
//	    },
//	  },
//	}
//
// ALGORITHM:
//  1. Split subdomain by '.' to get hierarchical levels
//  2. For each level, tokenize by hyphens and numbers:
//     - Split by '-' and prefix subsequent parts with '-'
//     - Further split by numbers using regex
//     - Special case: merge standalone '-' with following numbers
//
// This preserves the structure needed for pattern mining and clustering.
func Tokenize(subdomains []string) []TokenizedSubdomain {
	result := make([]TokenizedSubdomain, 0, len(subdomains))

	for _, subdomain := range subdomains {
		tokenized := TokenizedSubdomain{
			Original: subdomain,
			Levels:   []Level{}, // Initialize to empty slice, not nil
		}

		// Handle empty subdomains
		if subdomain == "" {
			result = append(result, tokenized)
			continue
		}

		// Split subdomain by '.' to get hierarchical labels
		labels := strings.Split(subdomain, ".")
		tokenized.Levels = make([]Level, 0, len(labels))

		for _, label := range labels {
			if label == "" {
				continue
			}
			level := Level{
				Label:  label,
				Tokens: tokenizeLabel(label),
			}
			tokenized.Levels = append(tokenized.Levels, level)
		}

		result = append(result, tokenized)
	}

	return result
}

// tokenizeLabel tokenizes a single label by splitting on hyphens and numbers.
//
// ALGORITHM:
//  1. Split by '-' and prefix subsequent parts with '-'
//  2. Split each part by numbers (e.g., "api12" → ["api", "12"])
//  3. Handle special case: standalone '-' followed by number becomes '-number'
//
// EXAMPLE:
//
//	"api-prod-12" → ["api", "-prod", "-12"]
//	"web01" → ["web", "01"]
//	"foo-12" → ["foo", "-12"]
func tokenizeLabel(label string) []string {
	tokens := make([]string, 0)

	// Split by hyphens and prefix subsequent parts with '-'
	hyphenParts := strings.Split(label, "-")
	for i, part := range hyphenParts {
		if part == "" {
			continue
		}

		// Prefix with '-' for all parts except the first
		if i != 0 {
			part = "-" + part
		}

		// Split by numbers using regex
		subtokens := splitByNumbers(part)

		// Handle special case: merge standalone '-' with following number
		// This happens when we have patterns like "foo-12"
		filtered := make([]string, 0, len(subtokens))
		for j, subtoken := range subtokens {
			if subtoken == "-" && j+1 < len(subtokens) {
				// If next token exists, merge with it
				if j+1 < len(subtokens) {
					subtokens[j+1] = "-" + subtokens[j+1]
				}
			} else {
				filtered = append(filtered, subtoken)
			}
		}

		tokens = append(tokens, filtered...)
	}

	return tokens
}

// numberSplitRegex is used to split strings by numeric sequences
var numberSplitRegex = regexp.MustCompile(`([0-9]+)`)

// splitByNumbers splits a string by numeric sequences while keeping the numbers.
//
// EXAMPLE:
//
//	"api12web34" → ["api", "12", "web", "34"]
//	"prod" → ["prod"]
//	"123" → ["123"]
func splitByNumbers(s string) []string {
	// Use regex to split by numbers but keep them in the result
	parts := numberSplitRegex.Split(s, -1)
	numbers := numberSplitRegex.FindAllString(s, -1)

	result := make([]string, 0, len(parts)+len(numbers))

	// Interleave parts and numbers
	numIndex := 0
	for i, part := range parts {
		if part != "" {
			result = append(result, part)
		}
		// Add the corresponding number if it exists
		if i < len(parts)-1 && numIndex < len(numbers) {
			result = append(result, numbers[numIndex])
			numIndex++
		}
	}

	return result
}

// extractFirstToken extracts the first token from a subdomain string.
// This is used for prefix-based clustering in the pattern mining algorithm.
//
// EXAMPLE:
//
//	"api-prod-1" → "api"
//	"web.dev" → "web"
//	"api5" → "api"
func (p *PatternMiner) extractFirstToken(subdomain string) string {
	if subdomain == "" {
		return ""
	}

	// Split by '.' to get the first level
	parts := strings.Split(subdomain, ".")
	if len(parts) == 0 {
		return ""
	}

	// Tokenize the first level
	tokens := tokenizeLabel(parts[0])
	if len(tokens) == 0 {
		return ""
	}

	// Return the first token, removing any hyphen prefix
	firstToken := tokens[0]
	return strings.TrimPrefix(firstToken, "-")
}
