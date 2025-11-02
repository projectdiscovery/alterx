package inducer

// TokenIndex provides efficient lookup and querying of tokens across all domains
// Structure: level → position → token_value → list of domain indices
// This enables fast queries like "find all domains with 'api' at level 0, position 0"
type TokenIndex struct {
	// index maps: level_index → position → token_value → []domain_indices
	index map[int]map[int]map[string][]int
}

// NewTokenIndex creates a new empty token index
func NewTokenIndex() *TokenIndex {
	return &TokenIndex{
		index: make(map[int]map[int]map[string][]int),
	}
}

// Add indexes a token from a domain at the specified level and position
// domainIndex is the position of the domain in the main domains array
func (ti *TokenIndex) Add(levelIndex, position int, tokenValue string, domainIndex int) {
	// Ensure level exists
	if ti.index[levelIndex] == nil {
		ti.index[levelIndex] = make(map[int]map[string][]int)
	}

	// Ensure position exists at this level
	if ti.index[levelIndex][position] == nil {
		ti.index[levelIndex][position] = make(map[string][]int)
	}

	// Add domain index to this token's list
	ti.index[levelIndex][position][tokenValue] = append(
		ti.index[levelIndex][position][tokenValue],
		domainIndex,
	)
}

// GetDomainIndices returns all domain indices that have the specified token
// at the given level and position
func (ti *TokenIndex) GetDomainIndices(levelIndex, position int, tokenValue string) []int {
	if ti.index[levelIndex] == nil {
		return []int{}
	}
	if ti.index[levelIndex][position] == nil {
		return []int{}
	}
	return ti.index[levelIndex][position][tokenValue]
}

// GetTokensAtLevelPosition returns all unique token values at a level and position
func (ti *TokenIndex) GetTokensAtLevelPosition(levelIndex, position int) []string {
	if ti.index[levelIndex] == nil {
		return []string{}
	}
	if ti.index[levelIndex][position] == nil {
		return []string{}
	}

	tokens := make([]string, 0, len(ti.index[levelIndex][position]))
	for token := range ti.index[levelIndex][position] {
		tokens = append(tokens, token)
	}
	return tokens
}

// GetLevelCount returns the number of levels indexed
func (ti *TokenIndex) GetLevelCount() int {
	return len(ti.index)
}

// LevelStats tracks statistics for a specific DNS level
type LevelStats struct {
	LevelIndex   int            // Which level (0-indexed)
	MaxPosition  int            // Highest position seen at this level
	TokenCounts  map[string]int // Token value → frequency count
	DomainCount  int            // Number of domains that have this level
	PositionDist map[int]int    // Position → count (how many domains have N positions)
}

// NewLevelStats creates statistics tracker for a level
func NewLevelStats(levelIndex int) *LevelStats {
	return &LevelStats{
		LevelIndex:   levelIndex,
		MaxPosition:  -1,
		TokenCounts:  make(map[string]int),
		PositionDist: make(map[int]int),
	}
}

// AddToken records a token occurrence at this level
func (ls *LevelStats) AddToken(position int, tokenValue string) {
	// Update max position
	if position > ls.MaxPosition {
		ls.MaxPosition = position
	}

	// Increment token count
	ls.TokenCounts[tokenValue]++
}

// IncrementDomainCount records that another domain has this level
func (ls *LevelStats) IncrementDomainCount(positionCount int) {
	ls.DomainCount++
	ls.PositionDist[positionCount]++
}

// GetTopTokens returns the N most frequent tokens at this level
func (ls *LevelStats) GetTopTokens(n int) []string {
	// Convert map to sortable slice
	type tokenCount struct {
		token string
		count int
	}

	tokens := make([]tokenCount, 0, len(ls.TokenCounts))
	for token, count := range ls.TokenCounts {
		tokens = append(tokens, tokenCount{token, count})
	}

	// Simple bubble sort for top N (good enough for small N)
	for i := 0; i < len(tokens); i++ {
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].count > tokens[i].count {
				tokens[i], tokens[j] = tokens[j], tokens[i]
			}
		}
	}

	// Return top N tokens
	result := []string{}
	for i := 0; i < n && i < len(tokens); i++ {
		result = append(result, tokens[i].token)
	}
	return result
}

// TokenStats provides global statistics about token types across all domains
type TokenStats struct {
	TotalTokens      int            // Total number of tokens across all domains
	TypeDistribution map[TokenType]int // Token type → count
	UniqueTokens     int            // Number of unique token values seen
	AvgTokensPerLevel float64       // Average number of tokens per level
}

// NewTokenStats creates a new token statistics tracker
func NewTokenStats() *TokenStats {
	return &TokenStats{
		TypeDistribution: make(map[TokenType]int),
	}
}

// AddToken records a token for statistics
func (ts *TokenStats) AddToken(tokenType TokenType) {
	ts.TotalTokens++
	ts.TypeDistribution[tokenType]++
}

// SetUniqueTokenCount updates the unique token count
func (ts *TokenStats) SetUniqueTokenCount(count int) {
	ts.UniqueTokens = count
}

// SetAvgTokensPerLevel updates the average tokens per level
func (ts *TokenStats) SetAvgTokensPerLevel(avg float64) {
	ts.AvgTokensPerLevel = avg
}
