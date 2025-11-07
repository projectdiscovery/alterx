package mining

// Edge represents a connection between two nodes or items
// in our case it is connection between two subdomains
type Edge [2]string

func NewEdge(sub1, sub2 string) Edge {
	if sub1 > sub2 {
		return Edge{sub2, sub1}
	}
	return Edge{sub1, sub2}
}

// Helper function to check if two clusters (as maps) are equal
func clustersEqual_internal(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// GenerateValidNgrams generates all valid unigrams and bigrams that can be used as
// PREFIX patterns for subdomain matching according to RFC 1123 rules.
//
// PURPOSE:
// These ngrams are used to find subdomains that START with specific patterns.
// For example, to find all subdomains starting with "a-", "ap", "1-", etc.
//
// RFC 1123 PREFIX RULES:
// - Valid characters: a-z, A-Z, 0-9, hyphen (-)
// - MUST start with: letter or digit (RFC 1123 requirement)
// - Second character can be: letter, digit, OR hyphen
//
// EXAMPLES:
//
//	Valid prefixes:   a, z, 0, 9, ab, a1, 1a, a-, 0-, api, web-
//	Invalid prefixes: -, -a, -0 (cannot start with hyphen)
//
// USE CASES:
//
//	"a"  matches: api.com, app.com, about.com
//	"a-" matches: a-one.com, a-test.com, a-api.com
//	"ab" matches: about.com, abc.com, abstract.com
//
// RETURNS:
//
//	unigrams: All valid single-character prefixes (a-z, A-Z, 0-9)
//	bigrams:  All valid two-character prefixes following RFC rules
func GenerateValidNgrams() (unigrams []string, bigrams []string) {
	// Valid characters for subdomain labels
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits := "0123456789"
	hyphen := "-"

	// Combine all valid characters
	allChars := letters + digits + hyphen
	// Characters that can start a subdomain (no hyphen)
	startChars := letters + digits

	// Generate all valid unigrams (single characters that can start a subdomain)
	// Hyphen cannot start a subdomain label
	for _, c := range startChars {
		unigrams = append(unigrams, string(c))
	}

	// Generate all valid bigrams (two-character prefixes)
	// Rule: Must start with letter/digit, second char can be anything valid
	for _, first := range startChars {
		for _, second := range allChars {
			bigrams = append(bigrams, string(first)+string(second))
		}
	}

	return unigrams, bigrams
}
