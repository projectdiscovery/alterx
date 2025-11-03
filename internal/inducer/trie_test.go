package inducer

import (
	"testing"
)

func TestNewTrie(t *testing.T) {
	domains := []string{
		"api.example.com",
		"api-dev.example.com",
		"admin.example.com",
		"web.example.com",
	}

	trie := NewTrie(domains)

	if trie == nil {
		t.Fatal("NewTrie returned nil")
	}

	if len(trie.Domains) != len(domains) {
		t.Errorf("Expected %d domains, got %d", len(domains), len(trie.Domains))
	}

	if trie.Root == nil {
		t.Fatal("Trie root is nil")
	}
}

func TestTrieInsertAndSearch(t *testing.T) {
	trie := &Trie{
		Root:    newTrieNode(),
		Domains: []string{"api.example.com", "api-dev.example.com", "admin.example.com"},
	}

	// Insert domains
	trie.Insert("api.example.com", 0)
	trie.Insert("api-dev.example.com", 1)
	trie.Insert("admin.example.com", 2)

	tests := []struct {
		name     string
		prefix   string
		expected []int
	}{
		{
			name:     "prefix 'api'",
			prefix:   "api",
			expected: []int{0, 1},
		},
		{
			name:     "prefix 'api.'",
			prefix:   "api.",
			expected: []int{0},
		},
		{
			name:     "prefix 'api-'",
			prefix:   "api-",
			expected: []int{1},
		},
		{
			name:     "prefix 'admin'",
			prefix:   "admin",
			expected: []int{2},
		},
		{
			name:     "prefix 'a'",
			prefix:   "a",
			expected: []int{0, 1, 2},
		},
		{
			name:     "non-existent prefix",
			prefix:   "xyz",
			expected: []int{},
		},
		{
			name:     "empty prefix",
			prefix:   "",
			expected: []int{0, 1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trie.SearchPrefix(tt.prefix)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("SearchPrefix(%q) returned %d results, expected %d", tt.prefix, len(result), len(tt.expected))
				return
			}

			// Check contents (convert to map for order-independent comparison)
			resultMap := make(map[int]bool)
			for _, id := range result {
				resultMap[id] = true
			}

			for _, expectedID := range tt.expected {
				if !resultMap[expectedID] {
					t.Errorf("SearchPrefix(%q) missing expected ID %d", tt.prefix, expectedID)
				}
			}
		})
	}
}

func TestGetNgramPrefixes(t *testing.T) {
	domains := []string{
		"api-dev.example.com",
		"api-prod.example.com",
		"admin.example.com",
		"web.example.com",
		"db01.example.com",
		"db02.example.com",
	}

	trie := NewTrie(domains)

	tests := []struct {
		name          string
		n             int
		expectedCount int // Number of prefix groups
		checkGroup    string
		expectedIDs   []int
	}{
		{
			name:          "1-gram (single char)",
			n:             1,
			expectedCount: 3, // a, w, d (api/admin share 'a', db01/db02 share 'd', web is 'w')
			checkGroup:    "a",
			expectedIDs:   []int{0, 1, 2}, // api-dev, api-prod, admin
		},
		{
			name:          "2-gram",
			n:             2,
			expectedCount: 4, // ap, ad, we, db
			checkGroup:    "ap",
			expectedIDs:   []int{0, 1}, // api-dev, api-prod
		},
		{
			name:          "3-gram",
			n:             3,
			expectedCount: 4, // api, adm, web, db0 (db01/db02 share 'db0')
			checkGroup:    "api",
			expectedIDs:   []int{0, 1}, // api-dev, api-prod
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trie.GetNgramPrefixes(tt.n)

			if len(result) != tt.expectedCount {
				t.Errorf("GetNgramPrefixes(%d) returned %d groups, expected %d", tt.n, len(result), tt.expectedCount)
			}

			// Check specific group
			groupIDs, exists := result[tt.checkGroup]
			if !exists {
				t.Errorf("Expected group %q not found", tt.checkGroup)
				return
			}

			if len(groupIDs) != len(tt.expectedIDs) {
				t.Errorf("Group %q has %d domains, expected %d", tt.checkGroup, len(groupIDs), len(tt.expectedIDs))
				return
			}

			// Verify IDs
			idMap := make(map[int]bool)
			for _, id := range groupIDs {
				idMap[id] = true
			}

			for _, expectedID := range tt.expectedIDs {
				if !idMap[expectedID] {
					t.Errorf("Group %q missing expected domain ID %d", tt.checkGroup, expectedID)
				}
			}
		})
	}
}

func TestGetTokenGroups(t *testing.T) {
	domains := []string{
		"api-dev.example.com",
		"api-prod.example.com",
		"admin.example.com",
		"web.example.com",
		"db01.example.com",
		"db02.example.com",
	}

	trie := NewTrie(domains)
	tokenGroups := trie.GetTokenGroups()

	tests := []struct {
		token       string
		expectedIDs []int
	}{
		{
			token:       "api",
			expectedIDs: []int{0, 1}, // api-dev, api-prod
		},
		{
			token:       "admin",
			expectedIDs: []int{2},
		},
		{
			token:       "web",
			expectedIDs: []int{3},
		},
		{
			token:       "db",
			expectedIDs: []int{4, 5}, // db01, db02
		},
	}

	for _, tt := range tests {
		t.Run("token_"+tt.token, func(t *testing.T) {
			groupIDs, exists := tokenGroups[tt.token]
			if !exists {
				t.Errorf("Token group %q not found", tt.token)
				return
			}

			if len(groupIDs) != len(tt.expectedIDs) {
				t.Errorf("Token group %q has %d domains, expected %d", tt.token, len(groupIDs), len(tt.expectedIDs))
				return
			}

			// Verify IDs
			idMap := make(map[int]bool)
			for _, id := range groupIDs {
				idMap[id] = true
			}

			for _, expectedID := range tt.expectedIDs {
				if !idMap[expectedID] {
					t.Errorf("Token group %q missing expected domain ID %d", tt.token, expectedID)
				}
			}
		})
	}
}

func TestGetPrefixGroups(t *testing.T) {
	domains := []string{
		"api-dev.example.com",
		"api-prod.example.com",
		"admin.example.com",
	}

	trie := NewTrie(domains)
	groups := trie.GetPrefixGroups(3) // 3-gram

	// Check "api" group
	apiGroup, exists := groups["api"]
	if !exists {
		t.Error("Expected 'api' prefix group not found")
		return
	}

	if len(apiGroup) != 2 {
		t.Errorf("'api' group has %d domains, expected 2", len(apiGroup))
	}

	// Verify domains are correct
	expectedDomains := map[string]bool{
		"api-dev.example.com":  true,
		"api-prod.example.com": true,
	}

	for _, domain := range apiGroup {
		if !expectedDomains[domain] {
			t.Errorf("Unexpected domain in 'api' group: %s", domain)
		}
	}
}

func TestGetTokenGroupDomains(t *testing.T) {
	domains := []string{
		"api-dev.example.com",
		"api-prod.example.com",
		"admin.example.com",
	}

	trie := NewTrie(domains)
	tokenGroups := trie.GetTokenGroupDomains()

	// Check "api" token group
	apiGroup, exists := tokenGroups["api"]
	if !exists {
		t.Error("Expected 'api' token group not found")
		return
	}

	if len(apiGroup) != 2 {
		t.Errorf("'api' token group has %d domains, expected 2", len(apiGroup))
	}

	// Verify domains
	expectedDomains := map[string]bool{
		"api-dev.example.com":  true,
		"api-prod.example.com": true,
	}

	for _, domain := range apiGroup {
		if !expectedDomains[domain] {
			t.Errorf("Unexpected domain in 'api' token group: %s", domain)
		}
	}
}

func TestTrieStats(t *testing.T) {
	domains := []string{
		"api.example.com",
		"api-dev.example.com",
		"admin.example.com",
	}

	trie := NewTrie(domains)
	stats := trie.GetStats()

	if stats.TotalDomains != len(domains) {
		t.Errorf("Stats.TotalDomains = %d, expected %d", stats.TotalDomains, len(domains))
	}

	if stats.TotalNodes <= 0 {
		t.Error("Stats.TotalNodes should be positive")
	}

	if stats.MaxDepth <= 0 {
		t.Error("Stats.MaxDepth should be positive")
	}

	// Check prefix groups
	for n := 1; n <= 3; n++ {
		if count, exists := stats.PrefixGroups[n]; !exists || count <= 0 {
			t.Errorf("Stats.PrefixGroups[%d] should exist and be positive", n)
		}
	}
}

func TestGetLongestCommonPrefix(t *testing.T) {
	tests := []struct {
		name     string
		domains  []string
		expected string
	}{
		{
			name:     "common prefix 'api'",
			domains:  []string{"api.example.com", "api-dev.example.com", "api-prod.example.com"},
			expected: "api",
		},
		{
			name:     "no common prefix",
			domains:  []string{"api.example.com", "web.example.com", "db.example.com"},
			expected: "",
		},
		{
			name:     "single domain",
			domains:  []string{"api.example.com"},
			expected: "api.example.com",
		},
		{
			name:     "empty domains",
			domains:  []string{},
			expected: "",
		},
		{
			name:     "full match",
			domains:  []string{"test.com", "test.com", "test.com"},
			expected: "test.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trie := NewTrie(tt.domains)
			result := trie.GetLongestCommonPrefix()

			if result != tt.expected {
				t.Errorf("GetLongestCommonPrefix() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestGetAllPrefixes(t *testing.T) {
	domains := []string{
		"api.example.com",
		"api-dev.example.com",
		"admin.example.com",
	}

	trie := NewTrie(domains)
	prefixes := trie.GetAllPrefixes()

	// Should return all 3 complete domains
	if len(prefixes) != 3 {
		t.Errorf("GetAllPrefixes() returned %d prefixes, expected 3", len(prefixes))
	}

	// Verify all domains are present
	prefixMap := make(map[string]bool)
	for _, prefix := range prefixes {
		prefixMap[prefix] = true
	}

	for _, domain := range domains {
		if !prefixMap[domain] {
			t.Errorf("GetAllPrefixes() missing domain %q", domain)
		}
	}
}

func TestTrieWithComplexDomains(t *testing.T) {
	// Test with real-world complex subdomain patterns
	domains := []string{
		"api-v1-prod.service.example.com",
		"api-v1-staging.service.example.com",
		"api-v2-prod.service.example.com",
		"web-frontend.service.example.com",
		"web-backend.service.example.com",
		"db-primary-01.data.example.com",
		"db-primary-02.data.example.com",
		"db-replica-01.data.example.com",
	}

	trie := NewTrie(domains)

	// Test 3-gram grouping
	groups := trie.GetNgramPrefixes(3)

	// "api" prefix should group api-v1-prod, api-v1-staging, api-v2-prod
	apiGroup, exists := groups["api"]
	if !exists {
		t.Error("Expected 'api' prefix group")
		return
	}

	if len(apiGroup) != 3 {
		t.Errorf("'api' group has %d domains, expected 3", len(apiGroup))
	}

	// "web" prefix should group web-frontend, web-backend
	webGroup, exists := groups["web"]
	if !exists {
		t.Error("Expected 'web' prefix group")
		return
	}

	if len(webGroup) != 2 {
		t.Errorf("'web' group has %d domains, expected 2", len(webGroup))
	}

	// "db-" prefix should group all db domains
	dbGroup, exists := groups["db-"]
	if !exists {
		t.Error("Expected 'db-' prefix group")
		return
	}

	if len(dbGroup) != 3 {
		t.Errorf("'db-' group has %d domains, expected 3", len(dbGroup))
	}
}

func TestTrieEmptyPrefix(t *testing.T) {
	domains := []string{
		"api.example.com",
		"web.example.com",
	}

	trie := NewTrie(domains)

	// Search with empty prefix should return all domains
	result := trie.SearchPrefix("")
	if len(result) != len(domains) {
		t.Errorf("SearchPrefix(\"\") returned %d domains, expected %d", len(result), len(domains))
	}
}

func TestTrieNgramWithShortDomains(t *testing.T) {
	domains := []string{
		"a.com",
		"ab.com",
		"abc.com",
		"abcd.com",
	}

	trie := NewTrie(domains)

	// 5-gram should group short domains together
	groups := trie.GetNgramPrefixes(5)

	// All domains shorter than 5 chars should use their full domain as prefix
	shortGroup1, exists := groups["a.com"]
	if exists && len(shortGroup1) != 1 {
		t.Error("Short domain 'a.com' should be in its own group")
	}

	shortGroup2, exists := groups["ab.com"]
	if exists && len(shortGroup2) != 1 {
		t.Error("Short domain 'ab.com' should be in its own group")
	}
}

func BenchmarkTrieInsert(b *testing.B) {
	domains := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		domains[i] = "subdomain-" + string(rune('a'+i%26)) + ".example.com"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie := NewTrie(domains)
		_ = trie
	}
}

func BenchmarkTrieSearchPrefix(b *testing.B) {
	domains := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		domains[i] = "subdomain-" + string(rune('a'+i%26)) + ".example.com"
	}

	trie := NewTrie(domains)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trie.SearchPrefix("subdomain-a")
	}
}

func BenchmarkTrieGetNgramPrefixes(b *testing.B) {
	domains := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		domains[i] = "subdomain-" + string(rune('a'+i%26)) + ".example.com"
	}

	trie := NewTrie(domains)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trie.GetNgramPrefixes(3)
	}
}
