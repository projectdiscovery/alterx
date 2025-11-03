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

// Closure represents a group of similar domains based on edit distance
// This is the core data structure for pattern generation in regulator algorithm
type Closure struct {
	Domains []string // The domains in this closure
	Delta   int      // The edit distance threshold used
	Size    int      // Number of domains
}
