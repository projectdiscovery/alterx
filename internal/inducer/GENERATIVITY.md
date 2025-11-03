# Generativity Estimation Implementation

## Overview

This document describes the implementation of regex generativity estimation for the pattern induction orchestrator. Generativity estimation calculates how many distinct strings a regex pattern can generate, which is essential for quality filtering during pattern learning.

## Location

- **Implementation**: `/internal/inducer/orchestrator.go` (lines 264-546)
- **Tests**: `/internal/inducer/generativity_test.go`

## Algorithm

The generativity estimation algorithm parses regex patterns and calculates the total number of possible combinations by multiplying the counts of various regex constructs:

### Supported Regex Constructs

1. **Alternations**: `(a|b|c)` → 3 options
   - Parses pipe-separated alternatives
   - Handles nested groups correctly
   - Sums generativity of each alternative

2. **Character Classes**: `[0-9]` → 10 options, `[a-z]` → 26 options
   - Expands character ranges
   - Handles individual characters
   - Supports escaped characters within classes

3. **Optional Groups**: `(...)? ` → multiply by (content_count + 1)
   - Accounts for "present" and "absent" states
   - Works with alternations: `(a|b)?` → 3 options (a, b, or absent)

4. **Escaped Characters**: `\.`, `-`, etc. → 1 option (literal)
   - Treats escaped characters as single literals

5. **Nested Groups**: Handles arbitrarily nested parentheses
   - Maintains depth tracking
   - Correctly matches opening/closing parens

## Key Functions

### Main Entry Point

```go
func (o *Orchestrator) estimateGenerativity(pattern *Pattern) int
```
- Takes a Pattern struct
- Returns the total number of possible strings the pattern can generate
- Used by `isGoodPattern()` for quality filtering

### Core Parser

```go
func estimateRegexGenerativity(regex string) int
```
- State machine-based parser
- Traverses regex string character by character
- Multiplies counts from all constructs

### Helper Functions

```go
func parseGroup(content string) int
```
- Parses group content (text between `(` and `)`)
- Handles alternations by splitting on `|`
- Recursively calculates generativity for each alternative

```go
func splitAlternatives(content string) []string
```
- Splits group content by top-level `|` characters
- Ignores `|` inside nested groups or character classes
- Handles escaped characters correctly

```go
func parseCharacterClass(content string) int
```
- Parses character class content (text between `[` and `]`)
- Expands ranges like `0-9` (10 chars) or `a-z` (26 chars)
- Handles mixed ranges and individual characters

```go
func findMatchingParen(s string, start int) int
func findClosingBracket(s string, start int) int
```
- Utility functions for finding matching delimiters
- Handle escaping and nesting correctly

## Examples

### Simple Alternation
```go
estimateRegexGenerativity("(api|web|cdn)")  // → 3
```

### Multiple Alternations
```go
estimateRegexGenerativity("(api|web)-(dev|prod)")  // → 4 (2 × 2)
```

### With Character Classes
```go
estimateRegexGenerativity("(db|cache)[0-9]")  // → 20 (2 × 10)
```

### Optional Groups
```go
estimateRegexGenerativity("api(\\.staging)?")  // → 2 (present or absent)
estimateRegexGenerativity("(api|web)?")  // → 3 (api, web, or absent)
```

### Complex Pattern
```go
estimateRegexGenerativity("(api|web|cdn)-(dev|prod|staging)[0-9][0-9]")
// → 900 (3 × 3 × 10 × 10)
```

### Real-World Pattern Generator Output
```go
// Pattern from multi-level domains with optional level
estimateRegexGenerativity("(db|cache|queue)[0-1][0-9](\\.(internal|external))?")
// → 180 (3 × 2 × 10 × 3)
// where last 3 = absent, .internal, .external
```

## Quality Filtering Integration

The generativity estimate is used in `isGoodPattern()` to filter out overly broad patterns:

```go
func (o *Orchestrator) isGoodPattern(pattern *Pattern) bool {
    generativity := o.estimateGenerativity(pattern)

    // Auto-accept small patterns
    if generativity < o.config.AbsoluteLimit {  // default: 500
        return true
    }

    // Ratio test for larger patterns
    ratio := float64(generativity) / float64(pattern.Coverage)
    return ratio < o.config.MaxRatio  // default: 25.0
}
```

### Quality Filtering Logic

1. **Absolute Limit**: Patterns generating < 500 possibilities are auto-accepted regardless of ratio
2. **Ratio Test**: For patterns above the limit, reject if `generativity/coverage > 25`

This ensures:
- Small, safe patterns are always accepted
- Large patterns must have good coverage to be accepted
- Prevents pattern explosion from overly generic patterns

## Testing

Comprehensive test suite in `generativity_test.go`:

- **TestEstimateRegexGenerativity**: 30+ test cases covering all regex constructs
- **TestParseCharacterClass**: Character class expansion tests
- **TestSplitAlternatives**: Alternation parsing tests
- **TestFindMatchingParen**: Parenthesis matching tests
- **TestFindClosingBracket**: Bracket matching tests
- **TestEstimateGenerativityWithPatternStruct**: Integration with Pattern struct
- **TestQualityFiltering**: Quality filtering logic tests

All tests pass with 100% coverage of the generativity estimation code.

## Edge Cases Handled

1. **Empty patterns**: Returns 1
2. **Nested groups**: Correctly tracks depth
3. **Escaped characters**: Treats as literals
4. **Character classes with ranges**: Correctly expands ranges
5. **Mixed escaped and regular characters**: Handles both
6. **Optional groups with alternations**: Correct math
7. **Invalid regex**: Fails safe (returns 1)

## Performance Considerations

- **Time Complexity**: O(n) where n is the length of the regex string
  - Single pass through the string
  - Recursive calls for nested groups are limited by string length

- **Space Complexity**: O(d) where d is the maximum nesting depth
  - Call stack for recursion
  - No large data structures allocated

- **Optimization**: Could be improved with:
  - Memoization for repeated patterns
  - Precomputed character class counts
  - But current performance is adequate for typical patterns

## Future Enhancements

Potential improvements:

1. **Quantifiers**: Support `{n,m}`, `+`, `*` operators
2. **Lookaheads/Lookbehinds**: Currently not used by pattern generator
3. **Backreferences**: Not needed for current use case
4. **Unicode ranges**: Support for non-ASCII character classes
5. **Caching**: Memoize results for repeated patterns

However, the current implementation handles all patterns generated by the pattern generator in `patterns.go`, which is the primary requirement.

## References

- Original regulator algorithm: Uses DankEncoder for generativity estimation
- Our implementation: Simpler, purpose-built for AlterX pattern format
- Pattern generator: `internal/inducer/patterns.go`
- Number compression: `internal/inducer/compression.go`
