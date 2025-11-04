package inducer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountLevels(t *testing.T) {
	tests := []struct {
		name               string
		domain             string
		expectedLevelCount int
		expectedRoot       string
		expectError        bool
	}{
		{
			name:               "single level subdomain",
			domain:             "api.projectdiscovery.io",
			expectedLevelCount: 1,
			expectedRoot:       "projectdiscovery.io",
			expectError:        false,
		},
		{
			name:               "two level subdomain",
			domain:             "scheduler.api.projectdiscovery.io",
			expectedLevelCount: 2,
			expectedRoot:       "projectdiscovery.io",
			expectError:        false,
		},
		{
			name:               "three level subdomain",
			domain:             "scheduler.v1.api.projectdiscovery.io",
			expectedLevelCount: 3,
			expectedRoot:       "projectdiscovery.io",
			expectError:        false,
		},
		{
			name:               "root domain only",
			domain:             "projectdiscovery.io",
			expectedLevelCount: 0,
			expectedRoot:       "projectdiscovery.io",
			expectError:        false,
		},
		{
			name:               "with dashes in subdomain",
			domain:             "api-dev.staging.example.com",
			expectedLevelCount: 2,
			expectedRoot:       "example.com",
			expectError:        false,
		},
		{
			name:               "with numbers",
			domain:             "db01.prod.internal.example.com",
			expectedLevelCount: 3,
			expectedRoot:       "example.com",
			expectError:        false,
		},
		{
			name:               "complex multi-level",
			domain:             "a.b.c.d.e.example.com",
			expectedLevelCount: 5,
			expectedRoot:       "example.com",
			expectError:        false,
		},
		{
			name:               "public suffix .co.uk",
			domain:             "api.example.co.uk",
			expectedLevelCount: 1,
			expectedRoot:       "example.co.uk",
			expectError:        false,
		},
		{
			name:               "two levels with .co.uk",
			domain:             "api.dev.example.co.uk",
			expectedLevelCount: 2,
			expectedRoot:       "example.co.uk",
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levelCount, root, err := CountLevels(tt.domain)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedLevelCount, levelCount, "level count mismatch")
				assert.Equal(t, tt.expectedRoot, root, "root mismatch")
			}
		})
	}
}

func TestGroupByLevelCount(t *testing.T) {
	tests := []struct {
		name           string
		domains        []string
		expectedGroups map[int][]string // levelCount → expected domains
	}{
		{
			name: "projectdiscovery.io example",
			domains: []string{
				"api.projectdiscovery.io",
				"cdn.projectdiscovery.io",
				"dev.projectdiscovery.io",
				"scheduler.api.projectdiscovery.io",
				"webhook.api.projectdiscovery.io",
				"webhook.dev.projectdiscovery.io",
				"scheduler.v1.api.projectdiscovery.io",
			},
			expectedGroups: map[int][]string{
				1: {
					"api.projectdiscovery.io",
					"cdn.projectdiscovery.io",
					"dev.projectdiscovery.io",
				},
				2: {
					"scheduler.api.projectdiscovery.io",
					"webhook.api.projectdiscovery.io",
					"webhook.dev.projectdiscovery.io",
				},
				3: {
					"scheduler.v1.api.projectdiscovery.io",
				},
			},
		},
		{
			name: "mixed environments",
			domains: []string{
				"api.example.com",
				"web.example.com",
				"api-dev.staging.example.com",
				"api-prod.staging.example.com",
				"web.prod.example.com",
				"db.prod.example.com",
			},
			expectedGroups: map[int][]string{
				1: {
					"api.example.com",
					"web.example.com",
				},
				2: {
					"api-dev.staging.example.com",
					"api-prod.staging.example.com",
					"web.prod.example.com",
					"db.prod.example.com",
				},
			},
		},
		{
			name: "all single level",
			domains: []string{
				"api.example.com",
				"web.example.com",
				"cdn.example.com",
			},
			expectedGroups: map[int][]string{
				1: {
					"api.example.com",
					"web.example.com",
					"cdn.example.com",
				},
			},
		},
		{
			name: "all same level count",
			domains: []string{
				"api.staging.example.com",
				"web.prod.example.com",
				"cdn.dev.example.com",
			},
			expectedGroups: map[int][]string{
				2: {
					"api.staging.example.com",
					"web.prod.example.com",
					"cdn.dev.example.com",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups, err := GroupByLevelCount(tt.domains)
			require.NoError(t, err)

			// Verify expected number of groups
			assert.Equal(t, len(tt.expectedGroups), len(groups), "wrong number of groups")

			// Verify each group
			for levelCount, expectedDomains := range tt.expectedGroups {
				group, exists := groups[levelCount]
				require.True(t, exists, "level group %d not found", levelCount)

				// Verify domains match (order doesn't matter)
				assert.ElementsMatch(t, expectedDomains, group.Domains,
					"domains mismatch for level %d", levelCount)

				// Verify level count is set correctly
				assert.Equal(t, levelCount, group.LevelCount,
					"level count mismatch in group")
			}
		})
	}
}

func TestGetSortedLevelGroups(t *testing.T) {
	// Create test groups
	groups := map[int]*LevelGroup{
		3: {
			LevelCount: 3,
			Domains:    []string{"a.b.c.example.com"},
		},
		1: {
			LevelCount: 1,
			Domains:    []string{"api.example.com"},
		},
		2: {
			LevelCount: 2,
			Domains:    []string{"api.dev.example.com"},
		},
	}

	sorted := GetSortedLevelGroups(groups)

	// Verify order: ascending by level count
	require.Len(t, sorted, 3)
	assert.Equal(t, 1, sorted[0].LevelCount, "first should be level 1")
	assert.Equal(t, 2, sorted[1].LevelCount, "second should be level 2")
	assert.Equal(t, 3, sorted[2].LevelCount, "third should be level 3")
}

func TestCountLevels_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		expectError bool
	}{
		{
			name:        "empty string",
			domain:      "",
			expectError: true,
		},
		{
			name:        "just TLD",
			domain:      ".com",
			expectError: true,
		},
		{
			name:        "invalid format",
			domain:      "not-a-domain",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := CountLevels(tt.domain)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGroupByLevelCount_EmptyInput(t *testing.T) {
	groups, err := GroupByLevelCount([]string{})
	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestGroupByLevelCount_SkipsRootDomains(t *testing.T) {
	domains := []string{
		"api.example.com",     // 1 level - should be included
		"example.com",         // 0 levels - should be skipped
		"cdn.example.com",     // 1 level - should be included
		"projectdiscovery.io", // 0 levels - should be skipped
	}

	groups, err := GroupByLevelCount(domains)
	require.NoError(t, err)

	// Should only have level 1 group
	assert.Len(t, groups, 1)
	assert.Contains(t, groups, 1)

	// Level 1 group should have 2 domains
	assert.Len(t, groups[1].Domains, 2)
	assert.ElementsMatch(t, []string{"api.example.com", "cdn.example.com"}, groups[1].Domains)
}
