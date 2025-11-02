package inducer

import (
	"fmt"
	"testing"
)

func TestTrie_InsertAndKeysWithPrefix(t *testing.T) {
	trie := NewTrie()

	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
		"web-staging.example.com",
		"web-prod.example.com",
	}

	for _, domain := range domains {
		trie.Insert(domain)
	}

	tests := []struct {
		prefix   string
		expected int
	}{
		{"api", 3},
		{"api-dev", 2},
		{"api-prod", 1},
		{"web", 2},
		{"web-staging", 1},
		{"xyz", 0}, // Non-existent prefix
		{"", 5},    // Empty prefix matches all
	}

	for _, tt := range tests {
		results := trie.KeysWithPrefix(tt.prefix)
		if len(results) != tt.expected {
			t.Errorf("KeysWithPrefix(%q) returned %d results; want %d", tt.prefix, len(results), tt.expected)
		}
	}
}

func TestTrie_Size(t *testing.T) {
	trie := NewTrie()

	if trie.Size() != 0 {
		t.Errorf("Empty trie size = %d; want 0", trie.Size())
	}

	trie.Insert("api")
	trie.Insert("web")
	trie.Insert("cdn")

	if trie.Size() != 3 {
		t.Errorf("Trie size = %d; want 3", trie.Size())
	}

	// Inserting duplicate doesn't increase size
	trie.Insert("api")
	if trie.Size() != 3 {
		t.Errorf("Trie size after duplicate = %d; want 3", trie.Size())
	}
}

func TestPartitioner_SmallDataset(t *testing.T) {
	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
		"web-staging.example.com",
	}

	partitioner := NewPartitioner(5000)
	groups := partitioner.Partition(domains)

	// With only 4 domains, should create minimal groups
	if len(groups) == 0 {
		t.Fatal("No groups created")
	}

	// Total domains should match input
	totalDomains := 0
	for _, group := range groups {
		totalDomains += group.Size
	}

	if totalDomains != len(domains) {
		t.Errorf("Total domains in groups = %d; want %d", totalDomains, len(domains))
	}

	// No group should exceed max size
	for _, group := range groups {
		if group.Size > 5000 {
			t.Errorf("Group %s has size %d; exceeds max 5000", group.Prefix, group.Size)
		}
	}
}

func TestPartitioner_LargeGroup(t *testing.T) {
	// Generate 10,000 domains with "api" prefix to force sub-partitioning
	domains := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		domains[i] = fmt.Sprintf("api-service-%04d.example.com", i)
	}

	partitioner := NewPartitioner(5000)
	groups := partitioner.Partition(domains)

	// Should create multiple groups
	if len(groups) < 2 {
		t.Errorf("Expected at least 2 groups for 10K domains, got %d", len(groups))
	}

	// Each group should be ≤ maxGroupSize
	for _, group := range groups {
		if group.Size > 5000 {
			t.Errorf("Group %s has size %d; exceeds max 5000", group.Prefix, group.Size)
		}
	}

	// Total domains should match input
	totalDomains := 0
	for _, group := range groups {
		totalDomains += group.Size
	}

	if totalDomains != len(domains) {
		t.Errorf("Total domains in groups = %d; want %d", totalDomains, len(domains))
	}
}

func TestPartitioner_MixedPrefixes(t *testing.T) {
	// Mix of different prefixes
	domains := []string{
		"api-dev.example.com",
		"api-prod.example.com",
		"web-staging.example.com",
		"web-prod.example.com",
		"cdn-us.example.com",
		"cdn-eu.example.com",
		"db-primary.example.com",
		"db-replica.example.com",
	}

	partitioner := NewPartitioner(5000)
	groups := partitioner.Partition(domains)

	// Should create groups for different prefixes
	if len(groups) == 0 {
		t.Fatal("No groups created")
	}

	// All domains should be accounted for
	totalDomains := 0
	for _, group := range groups {
		totalDomains += group.Size
	}

	if totalDomains != len(domains) {
		t.Errorf("Total domains in groups = %d; want %d", totalDomains, len(domains))
	}
}

func TestPartitioner_EmptyInput(t *testing.T) {
	partitioner := NewPartitioner(5000)
	groups := partitioner.Partition([]string{})

	if len(groups) != 0 {
		t.Errorf("Expected 0 groups for empty input, got %d", len(groups))
	}
}

func TestPartitioner_SingleDomain(t *testing.T) {
	partitioner := NewPartitioner(5000)
	groups := partitioner.Partition([]string{"api.example.com"})

	if len(groups) == 0 {
		t.Fatal("No groups created for single domain")
	}

	totalDomains := 0
	for _, group := range groups {
		totalDomains += group.Size
	}

	if totalDomains != 1 {
		t.Errorf("Expected 1 total domain, got %d", totalDomains)
	}
}

func TestPartitioner_CustomMaxSize(t *testing.T) {
	// Generate 300 domains
	domains := make([]string, 300)
	for i := 0; i < 300; i++ {
		domains[i] = fmt.Sprintf("api-%04d.example.com", i)
	}

	// Use max size of 100
	partitioner := NewPartitioner(100)
	groups := partitioner.Partition(domains)

	// Should create at least 3 groups (300 / 100)
	if len(groups) < 3 {
		t.Errorf("Expected at least 3 groups for 300 domains with max 100, got %d", len(groups))
	}

	// Each group should be ≤ 100
	for _, group := range groups {
		if group.Size > 100 {
			t.Errorf("Group %s has size %d; exceeds max 100", group.Prefix, group.Size)
		}
	}
}

func TestGenerate1Grams(t *testing.T) {
	partitioner := NewPartitioner(5000)
	grams := partitioner.generate1Grams()

	// Should have 26 letters + 10 digits = 36
	if len(grams) != 36 {
		t.Errorf("Expected 36 1-grams, got %d", len(grams))
	}

	// Check first few
	if grams[0] != "a" {
		t.Errorf("First 1-gram = %q; want %q", grams[0], "a")
	}

	if grams[25] != "z" {
		t.Errorf("26th 1-gram = %q; want %q", grams[25], "z")
	}

	if grams[26] != "0" {
		t.Errorf("27th 1-gram = %q; want %q", grams[26], "0")
	}
}

func TestGenerateNGrams(t *testing.T) {
	partitioner := NewPartitioner(5000)

	// Test 2-grams
	grams2 := partitioner.generateNGrams(2)
	if len(grams2) == 0 {
		t.Error("No 2-grams generated")
	}

	// Should have (26 letters + 10 digits + 1 dash) ^ 2 = 37^2 = 1369
	expected := 37 * 37
	if len(grams2) != expected {
		t.Errorf("Expected %d 2-grams, got %d", expected, len(grams2))
	}

	// Check first few 2-grams
	if grams2[0] != "aa" {
		t.Errorf("First 2-gram = %q; want %q", grams2[0], "aa")
	}
}

func BenchmarkPartitioner_1KDomains(b *testing.B) {
	domains := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		domains[i] = fmt.Sprintf("api-service-%04d.example.com", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		partitioner := NewPartitioner(5000)
		partitioner.Partition(domains)
	}
}

func BenchmarkPartitioner_10KDomains(b *testing.B) {
	domains := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		domains[i] = fmt.Sprintf("api-service-%05d.example.com", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		partitioner := NewPartitioner(5000)
		partitioner.Partition(domains)
	}
}

func BenchmarkTrie_Insert(b *testing.B) {
	domains := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		domains[i] = fmt.Sprintf("api-service-%04d.example.com", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie := NewTrie()
		for _, domain := range domains {
			trie.Insert(domain)
		}
	}
}
