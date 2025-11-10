# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AlterX is a fast and customizable subdomain wordlist generator using DSL (Domain-Specific Language). This is a **Go port** that integrates pattern mining capabilities from [Regulator](https://github.com/cramppet/regulator) by @cramppet into the original [ProjectDiscovery alterx](https://github.com/projectdiscovery/alterx).

**Key Features:**
- Template-based subdomain generation using variables like `{{sub}}`, `{{suffix}}`, `{{word}}`
- Pattern mining mode that automatically discovers subdomain patterns from observed data
- Three operation modes: default (user patterns), discover (mined patterns), both (combined)
- ClusterBomb attack pattern for generating permutations

## Build & Development Commands

```bash
# Build the binary
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Run linter (requires golangci-lint)
make lint

# Format code
make fmt

# Clean build artifacts
make clean

# Install to $GOPATH/bin
make install

# Build and run help
make run
```

**Single test execution:**
```bash
go test -v -run TestFunctionName ./path/to/package
```

## Architecture

### Core Components

**1. Entry Point** (`cmd/alterx/main.go`)
- CLI argument parsing via `runner.ParseFlags()`
- Mode selection logic (default/discover/both)
- Pattern mining flow orchestration
- Deduplication between mined and user-defined patterns

**2. Mutator Engine** (`mutator.go`, `algo.go`)
- `Mutator` struct: Core permutation generator
- `ClusterBomb` algorithm: Nth-order payload combination using recursion
- `IndexMap`: Maintains deterministic ordering for payload iteration
- Template replacement using variables extracted from input domains

**3. Input Processing** (`inputs.go`)
- `Input` struct: Parses domains into components (sub, suffix, tld, etld, etc.)
- Variable extraction: `{{sub}}`, `{{sub1}}`, `{{suffix}}`, `{{root}}`, `{{sld}}`, etc.
- Multi-level subdomain support (e.g., `cloud.api.example.com` → `sub=cloud`, `sub1=api`)

**4. Pattern Mining** (`internal/patternmining/`)
- **Three-phase discovery algorithm:**
  1. Edit distance clustering (no prefix enforcement)
  2. N-gram clustering (unigrams/bigrams)
  3. N-gram prefix clustering with edit distance refinement
- **Quality control:** Pattern threshold and quality ratio prevent over-generation
- **Regex generation:** Converts clusters to patterns with alternations `(a|b)` and optional groups `(...)?`
- **Number compression:** Optimizes `[0-9]` ranges automatically

**5. DFA Engine** (`internal/dank/dank.go`)
- Brzozowski's algorithm for DFA minimization
- Thompson NFA construction from regex
- Subset construction for NFA→DFA conversion
- Reverse DFA for minimization (determinize → reverse → determinize → reverse → determinize)
- Fixed-length string generation from automaton

### File Structure

```
cmd/alterx/main.go          # Entry point, mode selection, orchestration
internal/runner/
  ├── runner.go             # CLI flag definitions and parsing
  ├── config.go             # Version and config management
  └── banner.go             # Banner display
internal/patternmining/
  ├── patternmining.go      # Main mining algorithm (3 phases)
  ├── clustering.go         # Edit distance clustering logic
  └── regex.go              # Tokenization and regex generation
internal/dank/
  └── dank.go               # DFA-based pattern generation (Brzozowski)
mutator.go                  # Core Mutator with ClusterBomb algorithm
algo.go                     # ClusterBomb implementation and IndexMap
inputs.go                   # Domain parsing and variable extraction
replacer.go                 # Template variable replacement
config.go                   # Default patterns and payloads
util.go                     # Helper functions
```

## Key Concepts

### Variables System
Templates use variables extracted from input domains:
- `{{sub}}`: Leftmost subdomain part (e.g., `api` in `api.example.com`)
- `{{suffix}}`: Everything except leftmost part (e.g., `example.com`)
- `{{root}}`: eTLD+1 (e.g., `example.com`)
- `{{sld}}`: Second-level domain (e.g., `example`)
- `{{tld}}`: Top-level domain (e.g., `com`)
- `{{etld}}`: Extended TLD (e.g., `co.uk`)
- `{{subN}}`: Multi-level support where N is depth (e.g., `{{sub1}}`, `{{sub2}}`)

### ClusterBomb Algorithm
Generates all combinations of payloads across variables:
- Uses recursion with vector construction
- Maintains deterministic ordering via IndexMap
- Avoids redundant combinations (e.g., `api-api.example.com`)
- Early exit when no variables present in template

### Pattern Mining Workflow
1. **Validate input:** Ensure domains share common target (e.g., `.example.com`)
2. **Build distance table:** Compute pairwise Levenshtein distances
3. **Phase 1 - Edit clustering:** Group by edit distance (min to max)
4. **Phase 2 - N-grams:** Generate unigrams/bigrams, cluster by prefix
5. **Phase 3 - Prefix clustering:** Apply edit distance within prefix groups
6. **Quality validation:** Filter patterns using threshold and ratio metrics
7. **Generate subdomains:** Use DFA to produce strings from patterns

## Pattern Mining Modes

**Default Mode** (`-m default` or omit):
- Original alterx behavior
- Uses user-defined or default patterns from config

**Discover Mode** (`-m discover`):
- Pattern mining only
- Discovers patterns from input domains
- Generates subdomains based only on mined patterns

**Both Mode** (`-m both`):
- Combines user-defined and mined patterns
- Deduplicates results across both sources
- Best for maximum coverage

**Key Flags:**
- `-min-distance 2`: Minimum Levenshtein distance for clustering
- `-max-distance 10`: Maximum Levenshtein distance for clustering
- `-pattern-threshold 500`: Minimum synthetic subdomains before ratio check
- `-quality-ratio 25`: Max ratio of synthetic/observed subdomains
- `-save-rules output.json`: Save discovered patterns and metadata to JSON file

## Common Patterns

### Adding New CLI Flags
1. Add field to `Options` struct in `internal/runner/runner.go`
2. Register flag in `ParseFlags()` using appropriate flag group
3. Handle flag value in main logic (`cmd/alterx/main.go`)

### Adding New Variables
1. Parse in `NewInput()` in `inputs.go`
2. Add to `Input.GetMap()` return value
3. Update template validation in `mutator.go`

### Modifying Pattern Mining
- **Clustering logic:** `internal/patternmining/clustering.go`
- **Tokenization rules:** `tokenize()` in `internal/patternmining/regex.go`
- **Quality metrics:** `isGoodRule()` in `internal/patternmining/patternmining.go`

## Testing Strategy

- Unit tests in `*_test.go` files (e.g., `mutator_test.go`, `inputs_test.go`)
- Test individual components before integration
- Use table-driven tests for variable extraction and pattern generation
- Validate pattern mining with known domain sets

## Important Notes

- **Dedupe enabled by default:** `DedupeResults = true` in `mutator.go`
- **Prefix optimization:** ClusterBomb skips words already in leftmost subdomain
- **Pattern quality critical:** Low thresholds generate millions of subdomains
- **Distance memoization:** Pattern mining caches Levenshtein distances for performance
- **DFA minimization:** Three-pass Brzozowski ensures minimal automaton
- **No breaking changes:** All pattern mining is additive; default behavior unchanged

## Credits

- **Original alterx:** [ProjectDiscovery](https://github.com/projectdiscovery/alterx)
- **Pattern mining algorithm:** [Regulator](https://github.com/cramppet/regulator) by @cramppet
- **DFA implementation:** Ported from original regulator/dank library

## Development Guidelines

- Maintain compatibility with original alterx API
- Keep pattern mining as optional feature (don't force on users)
- Preserve deterministic output ordering for testing
- Use `gologger` for all logging (not fmt.Println)
- Follow Go naming conventions and project structure
- Add tests for new features
