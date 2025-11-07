# Pattern Mining Feature

## Overview

Pattern mining allows alterx to automatically discover patterns from a list of input domains, eliminating the need to manually define patterns and payloads.

## Usage

### Basic Discover Mode

```bash
# Discover patterns from a list of domains
alterx -l domains.txt -d

# Limit the output
alterx -l domains.txt -d -limit 100
```

### Advanced Options

All pattern mining options are grouped under the **"Pattern Mining"** flag group:

| Flag | Default | Description |
|------|---------|-------------|
| `-d, -discover` | `false` | Enable discover mode (automatic pattern mining) |
| `-min-distance` | `2` | Minimum levenshtein distance for clustering |
| `-max-distance` | `5` | Maximum levenshtein distance for clustering |
| `-pattern-threshold` | `1000` | Pattern threshold for filtering low-quality patterns |
| `-quality-ratio` | `100` | Pattern quality ratio threshold |
| `-ngrams-limit` | `0` | Limit number of n-grams to process (0 = all) |

### Examples

**1. Basic discover mode:**
```bash
alterx -l subdomains.txt -d -limit 50
```

**2. Custom mining parameters:**
```bash
alterx -l subdomains.txt -d \
  -min-distance 3 \
  -max-distance 6 \
  -pattern-threshold 500 \
  -quality-ratio 80 \
  -limit 100
```

**3. Fast mode (limit n-grams):**
```bash
# Process only first 100 n-grams for faster results
alterx -l subdomains.txt -d -ngrams-limit 100
```

**4. Discover and save to file:**
```bash
alterx -l subdomains.txt -d -o permutations.txt
```

## Input Requirements

For optimal pattern discovery:
- **Minimum**: 10 domains (warning shown if fewer)
- **Recommended**: 50+ domains for better pattern diversity
- **Best**: 100+ domains with varied structures

## How It Works

The pattern mining algorithm uses two complementary approaches:

1. **Levenshtein Distance Clustering**: Groups similar subdomains based on edit distance
2. **Hierarchical N-gram Clustering**: Analyzes subdomains at multiple granularity levels

### Example

Given input domains:
```
api-prod.example.com
api-staging.example.com
web-prod.example.com
web-staging.example.com
```

Discovered patterns:
```
api-{{p0}}.{{root}}     → payloads: {"p0": ["prod", "staging"]}
web-{{p0}}.{{root}}     → payloads: {"p0": ["prod", "staging"]}
{{p0}}.{{root}}         → payloads: {"p0": ["api-prod", "api-staging", "web-prod", "web-staging"]}
```

Generated permutations:
```
api-prod.example.com
api-staging.example.com
web-prod.example.com
web-staging.example.com
(and many more combinations...)
```

## Architecture

The implementation uses a clean interface-based design:

- **`PatternProvider`** interface: Common contract for pattern generation strategies
- **`ManualPatternProvider`**: Traditional mode with user-specified patterns
- **`MinedPatternProvider`**: Discover mode with automatic pattern mining
- **Mutator**: Uses patterns/payloads from provider transparently

## Backward Compatibility

Manual mode remains unchanged:
```bash
# Traditional usage still works exactly as before
alterx -l domains.txt -p "{{word}}.{{root}}" -pp 'word=words.txt'
```

## Performance Tuning

### For Large Datasets (1000+ domains)

```bash
# Reduce distance ranges
alterx -l large-list.txt -d -min-distance 2 -max-distance 4

# Limit n-grams for faster processing
alterx -l large-list.txt -d -ngrams-limit 200
```

### For Quality over Speed

```bash
# Process all n-grams with strict thresholds
alterx -l domains.txt -d \
  -ngrams-limit 0 \
  -pattern-threshold 2000 \
  -quality-ratio 150
```

## Testing

Run pattern mining tests:
```bash
# Unit tests
go test -v -run TestMinedPatternProvider

# Integration tests
go test -v -run TestMutatorIntegration_DiscoverMode

# Cross-validation tests (requires Python)
cd mining && go test -v -run TestPatternDifferences
```

## Algorithm Details

See [mining/README.md](mining/README.md) for detailed algorithm documentation and Python reference implementation comparison.
