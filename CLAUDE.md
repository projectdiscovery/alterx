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

# Run specific test at package root
go test -v -run TestMutator
go test -v -run TestInput

# Run with race detector
go test -race -v ./...
```

## Architecture

### Core Components

**1. Entry Point** (`cmd/alterx/main.go`)
- CLI argument parsing via `runner.ParseFlags()` using goflags library
- Mode selection logic (default/discover/both) passed to `alterx.Options`
- Pattern mining flow orchestration in `Mutator.Execute()` via goroutines
- Output writing with `getOutputWriter()` (file or stdout)
- Rules saving via `Mutator.SaveRules()` after execution completes

**2. Mutator Engine** (`mutator.go`, `algo.go`)
- `Mutator` struct: Core permutation generator with concurrent execution
- `Execute()` method: Runs default and/or mining modes in parallel goroutines
- `ClusterBomb` algorithm: Recursive Nth-order payload combination (cartesian product)
- `IndexMap`: Maintains deterministic ordering for payload iteration
- Template replacement using `fasttemplate` library with `{{var}}` syntax
- Deduplication via `dedupe.NewDedupe()` with configurable memory limits
- Smart optimization: Skips words already present in leftmost subdomain

**3. Input Processing** (`inputs.go`)
- `Input` struct: Parses domains into components (sub, suffix, tld, etld, etc.)
- Uses `publicsuffix` library to extract eTLD and root domain correctly
- Variable extraction: `{{sub}}`, `{{sub1}}`, `{{suffix}}`, `{{root}}`, `{{sld}}`, etc.
- Multi-level subdomain support (e.g., `cloud.api.example.com` → `sub=cloud`, `sub1=api`)
- `getNValidateRootDomain()`: Validates homogeneous domains for pattern mining

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
1. **Validate input:** `getNValidateRootDomain()` ensures domains share common root
2. **Build distance table:** Compute pairwise Levenshtein distances with memoization
3. **Phase 1 - Edit clustering:** Group by edit distance (min to max) without prefix enforcement
4. **Phase 2 - N-grams:** Generate unigrams/bigrams, cluster by prefix
5. **Phase 3 - Prefix clustering:** Apply edit distance within prefix groups for refinement
6. **Quality validation:** `isGoodRule()` filters patterns using threshold and ratio metrics
7. **Regex generation:** Convert clusters to regex with alternations `(a|b)` and optional groups
8. **Generate subdomains:** DFA engine produces fixed-length strings from patterns

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

## Execution Flow

### Mode-Based Execution
The `Mutator.Execute()` method orchestrates parallel execution based on mode:

**Default Mode:**
1. Parse inputs → Extract variables → Validate patterns
2. Optionally enrich payloads from input subdomains
3. For each input × pattern combination:
   - Replace input variables (e.g., `{{sub}}`, `{{suffix}}`)
   - Execute ClusterBomb for payload permutations
   - Skip patterns with missing variables
4. Deduplicate results and write to output

**Discover Mode:**
1. Validate homogeneous domains (must share root)
2. Initialize `Miner` with distance/quality parameters
3. Run three-phase clustering algorithm
4. Generate regex patterns from clusters
5. Use DFA engine to produce subdomains
6. Skip input domains from output (avoid duplicates)

**Both Mode:**
- Runs default and discover in parallel goroutines
- Deduplication happens at channel level
- Results combined before writing

### Key Variables & Utilities

**Variable Extraction (`util.go`):**
- `getAllVars()`: Extract variable names from template using regex
- `checkMissing()`: Validate all variables have values before execution
- `getSampleMap()`: Merge input variables with payload variables for validation
- `unsafeToBytes()`: Zero-allocation string→byte conversion for performance

**Deduplication:**
- Enabled by default (`DedupeResults = true` in `mutator.go`)
- Uses memory-efficient dedupe from projectdiscovery/utils
- Estimates required memory: `count * maxkeyLenInBytes`

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
- **DFA operations:** `internal/dank/dank.go` (minimize, generate strings)

### Working with Modes
When adding features that interact with modes:
1. Check `opts.Mode` in `New()` to conditionally initialize components
2. Use goroutines in `Execute()` for parallel execution (default + discover)
3. Remember to close channels properly in `Execute()` cleanup goroutine
4. Mode validation happens in `Options.Validate()` with backwards-compatible defaults

## Testing Strategy

- Unit tests in `*_test.go` files (e.g., `mutator_test.go`, `inputs_test.go`)
- Test individual components before integration
- Use table-driven tests for variable extraction and pattern generation
- Validate pattern mining with known domain sets

## Important Notes

- **Dedupe enabled by default:** `DedupeResults = true` in `mutator.go`
- **Prefix optimization:** ClusterBomb skips words already in leftmost subdomain (lines 378-387 in `mutator.go`)
- **Pattern quality critical:** Low thresholds generate millions of subdomains
- **Distance memoization:** Pattern mining caches Levenshtein distances for performance in `Miner.memo` map
- **DFA minimization:** Three-pass Brzozowski ensures minimal automaton
- **No breaking changes:** All pattern mining is additive; default behavior unchanged
- **SaveRules timing:** Must be called AFTER `Execute()` to ensure mining completes (line 68-72 in `cmd/alterx/main.go`)
- **Homogeneous domains required:** Discover/both modes validate all domains share same root via `getNValidateRootDomain()`
- **Goroutine-safe:** Pattern mining and default mode run in separate goroutines with WaitGroup coordination

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
- Use `fasttemplate` for variable replacement (already integrated)
- Respect memory limits via `MaxSize` option in output writing

## Common Gotchas & Troubleshooting

### Pattern Mining Issues
- **"domains do not have the same root"**: All input domains must share a common root (e.g., all under `.example.com`). Use `getNValidateRootDomain()` to validate.
- **Too many patterns generated**: Decrease `-max-distance` or increase `-pattern-threshold` and `-quality-ratio`
- **No patterns discovered**: Increase `-max-distance` or decrease `-min-distance` to allow more clustering

### ClusterBomb Performance
- **Memory exhaustion**: Reduce payload sizes or use `-limit` to cap output
- **Slow execution**: Check that prefix optimization is working (should skip redundant words)
- **Expected combinations not appearing**: Verify variables exist in pattern template and payload map

### Mode Selection
- **Default mode** works without any special validation (backwards compatible)
- **Discover/Both modes** require homogeneous domains (same root)
- **SaveRules only works** with discover/both modes after execution completes

### Testing Tips
- Use `DryRun()` or `EstimateCount()` to validate logic without generating output
- Test pattern mining with small domain sets first (5-10 domains)
- For ClusterBomb testing, use simple 2-variable patterns to verify cartesian product logic
