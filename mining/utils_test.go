package mining

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateValidNgrams(t *testing.T) {
	unigrams, bigrams := GenerateValidNgrams()

	t.Run("Unigrams", func(t *testing.T) {
		// Should have 52 letters (a-z, A-Z) + 10 digits (0-9) = 62 unigrams
		require.Len(t, unigrams, 62, "Should have 62 valid unigrams")

		// Check that all expected characters are present
		expectedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for _, c := range expectedChars {
			assert.Contains(t, unigrams, string(c), "Should contain %s", string(c))
		}

		// Verify no hyphen in unigrams (hyphen cannot be standalone)
		assert.NotContains(t, unigrams, "-", "Hyphen should not be a valid unigram")

		// Verify all are single character
		for _, u := range unigrams {
			assert.Len(t, u, 1, "Unigram should be single character: %s", u)
		}
	})

	t.Run("Bigrams", func(t *testing.T) {
		// Should have 62 * 63 = 3,906 bigrams
		// (62 start chars) * (63 second chars: 62 alphanumeric + 1 hyphen)
		require.Len(t, bigrams, 62*63, "Should have 3,906 valid bigrams")

		// Verify all are two characters
		for _, b := range bigrams {
			assert.Len(t, b, 2, "Bigram should be two characters: %s", b)
		}

		// Check valid PREFIX examples are present
		validPrefixes := []string{
			"aa", "ab", "a1", "1a", "0z",
			"AA", "Ab", "A1", "1A", "0Z",
			"a-", "1-", "z-", // Valid prefixes for subdomains like a-one.com
		}

		for _, prefix := range validPrefixes {
			assert.Contains(t, bigrams, prefix, "Valid prefix %s should be present", prefix)
		}

		// Check invalid prefixes are NOT present
		invalidPrefixes := []string{
			"-a", "-1", "--", // Cannot start with hyphen
		}

		for _, prefix := range invalidPrefixes {
			assert.NotContains(t, bigrams, prefix, "Invalid prefix %s should not be present", prefix)
		}
	})

	t.Run("RFC_Compliance", func(t *testing.T) {
		// Test that no ngram starts with hyphen (RFC 1123 requirement)
		allNgrams := append(unigrams, bigrams...)

		for _, ngram := range allNgrams {
			assert.False(t, strings.HasPrefix(ngram, "-"),
				"Ngram should not start with hyphen: %s", ngram)
		}
	})

	t.Run("Bigrams_With_Hyphen", func(t *testing.T) {
		// Bigrams CAN end with hyphen as they are PREFIXES
		// For example, "a-" is a valid prefix for "a-one.com"
		hyphensFound := 0
		for _, b := range bigrams {
			if strings.HasSuffix(b, "-") {
				hyphensFound++
			}
		}
		// Should have 62 bigrams ending with hyphen (one for each start char)
		assert.Equal(t, 62, hyphensFound, "Should have 62 bigrams ending with hyphen")
	})

	t.Run("Character_Validation", func(t *testing.T) {
		validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-"
		allNgrams := append(unigrams, bigrams...)

		for _, ngram := range allNgrams {
			for _, c := range ngram {
				assert.Contains(t, validChars, string(c),
					"Ngram %s contains invalid character: %c", ngram, c)
			}
		}
	})
}

func TestGenerateValidNgrams_Uniqueness(t *testing.T) {
	unigrams, bigrams := GenerateValidNgrams()

	t.Run("Unigrams_Unique", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, u := range unigrams {
			assert.False(t, seen[u], "Duplicate unigram found: %s", u)
			seen[u] = true
		}
	})

	t.Run("Bigrams_Unique", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, b := range bigrams {
			assert.False(t, seen[b], "Duplicate bigram found: %s", b)
			seen[b] = true
		}
	})
}

func TestGenerateValidNgrams_Examples(t *testing.T) {
	unigrams, bigrams := GenerateValidNgrams()

	testCases := []struct {
		name      string
		ngram     string
		isValid   bool
		isUnigram bool
	}{
		// Valid unigrams
		{"lowercase letter", "a", true, true},
		{"uppercase letter", "Z", true, true},
		{"digit", "5", true, true},

		// Invalid unigrams
		{"hyphen alone", "-", false, true},

		// Valid bigrams (prefixes)
		{"two letters", "ab", true, false},
		{"letter then digit", "a1", true, false},
		{"digit then letter", "1a", true, false},
		{"two digits", "99", true, false},
		{"mixed case", "Aa", true, false},
		{"letter then hyphen", "a-", true, false}, // Valid prefix for a-one.com
		{"digit then hyphen", "1-", true, false},  // Valid prefix for 1-api.com

		// Invalid bigrams
		{"starts with hyphen", "-a", false, false},
		{"two hyphens", "--", false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var found bool
			if tc.isUnigram {
				for _, u := range unigrams {
					if u == tc.ngram {
						found = true
						break
					}
				}
			} else {
				for _, b := range bigrams {
					if b == tc.ngram {
						found = true
						break
					}
				}
			}

			if tc.isValid {
				assert.True(t, found, "Expected valid ngram %s to be present", tc.ngram)
			} else {
				assert.False(t, found, "Expected invalid ngram %s to be absent", tc.ngram)
			}
		})
	}
}

func BenchmarkGenerateValidNgrams(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateValidNgrams()
	}
}
