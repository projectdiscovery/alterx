package inducer

// TokenType represents the classification of a token
type TokenType int

const (
	// TokenTypeWord represents alphabetic tokens (e.g., "api", "web")
	TokenTypeWord TokenType = iota
	// TokenTypeDash represents dash-prefixed tokens (e.g., "-dev", "-prod")
	TokenTypeDash
	// TokenTypeNumber represents numeric tokens (e.g., "01", "123")
	TokenTypeNumber
)

// String returns string representation of TokenType
func (t TokenType) String() string {
	switch t {
	case TokenTypeWord:
		return "word"
	case TokenTypeDash:
		return "dash"
	case TokenTypeNumber:
		return "number"
	default:
		return "unknown"
	}
}

// Token represents a single token within a subdomain level
// Tokens preserve structural information like dash prefixes and number sequences
type Token struct {
	Value    string    // The actual token text (e.g., "api", "-dev", "01")
	Type     TokenType // Classification of the token
	Position int       // Position within the level (0-indexed internally)
}

// Level represents all tokens at a specific DNS hierarchy level
// Levels are indexed from 0 (leftmost/first subdomain level) to N
type Level struct {
	Index  int     // 0-indexed internally (level 0, level 1, level 2...)
	Tokens []Token // Ordered list of tokens at this level
}

// TokenizedDomain represents a fully parsed and tokenized subdomain
// This structure preserves the original domain while providing structured access
// to its components following the regulator tokenization algorithm
type TokenizedDomain struct {
	Original  string  // Original full domain (e.g., "api-dev-01.staging.example.com")
	Subdomain string  // Subdomain part only (e.g., "api-dev-01.staging")
	Root      string  // Root domain/eTLD+1 (e.g., "example.com")
	Levels    []Level // Parsed levels in order (0-indexed internally)
}

// GetLevelCount returns the number of levels in this tokenized domain
func (td *TokenizedDomain) GetLevelCount() int {
	return len(td.Levels)
}

// GetLevel returns the level at the specified index (0-indexed)
// Returns nil if the index is out of bounds
func (td *TokenizedDomain) GetLevel(index int) *Level {
	if index < 0 || index >= len(td.Levels) {
		return nil
	}
	return &td.Levels[index]
}

// GetTokensAtLevel returns all tokens at the specified level index (0-indexed)
// Returns nil if the level doesn't exist
func (td *TokenizedDomain) GetTokensAtLevel(levelIndex int) []Token {
	level := td.GetLevel(levelIndex)
	if level == nil {
		return nil
	}
	return level.Tokens
}

// GetToken returns a specific token at level and position (both 0-indexed)
// Returns nil if either index is out of bounds
func (td *TokenizedDomain) GetToken(levelIndex, position int) *Token {
	tokens := td.GetTokensAtLevel(levelIndex)
	if tokens == nil || position < 0 || position >= len(tokens) {
		return nil
	}
	return &tokens[position]
}

// InducerStats provides summary statistics about the pattern inducer
type InducerStats struct {
	TotalDomains     int            // Total number of input domains
	TokenizedDomains int            // Successfully tokenized domains
	FailedDomains    int            // Failed to tokenize (invalid format, etc.)
	MaxLevels        int            // Maximum number of levels seen
	LevelCounts      map[int]int    // Distribution: level_count → domain_count
	TokenTypeStats   map[TokenType]int // Token type distribution
}
