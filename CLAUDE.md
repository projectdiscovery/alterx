# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**AlterX** is a fast, customizable subdomain wordlist generator using Domain-Specific Language (DSL) patterns. It generates subdomain permutations based on input domains and configurable patterns, primarily used for security research and penetration testing. It's part of the ProjectDiscovery security toolkit ecosystem.

**Repository**: `github.com/projectdiscovery/alterx`
**Language**: Go 1.23.0
**License**: MIT

## Development Commands

### Build and Run
```bash
# Install the tool locally
go install github.com/projectdiscovery/alterx/cmd/alterx@latest

# Run directly from source
go run cmd/alterx/main.go -l domains.txt

# Build binary
go build -o alterx cmd/alterx/main.go

# Run programmatic example
go run examples/main.go
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run specific test
go test -run TestMutator

# Run tests for specific package
go test ./inducer/
go test -v -run Example ./inducer/

# Race condition test with live input
echo "www.scanme.sh" | go run -race cmd/alterx/main.go
```

### Linting
```bash
# The project uses golangci-lint (configured in GitHub Actions)
golangci-lint run --timeout=5m
```

## Architecture Overview

### Core Components

The codebase follows a **template-based permutation engine** architecture:

```
Input Domains → Parse Variables → Apply Patterns → ClusterBomb Algorithm → Deduplicate → Output
```

**Key files and their responsibilities:**

1. **`mutator.go`** (~330 lines): Core permutation engine
   - `Mutator` struct: Main orchestrator
   - `Execute()`: Streams results via Go channels
   - `ExecuteWithWriter()`: Direct output to io.Writer
   - `enrichPayloads()`: Extracts words from input domains

2. **`inputs.go`** (~100 lines): Domain parsing and variable extraction
   - `Input` struct: Represents parsed domain with all variables
   - Extracts: `{{sub}}`, `{{suffix}}`, `{{tld}}`, `{{etld}}`, `{{sld}}`, `{{root}}`, `{{subN}}`
   - Uses `golang.org/x/net/publicsuffix` for accurate TLD detection

3. **`algo.go`** (~83 lines): ClusterBomb algorithm implementation
   - Generates Cartesian product of payload sets
   - Optimization: Skips redundant combinations (e.g., won't generate `api-api.com`)
   - Uses recursive approach with callback for streaming

4. **`config.go`** (~58 lines): Configuration file handling
   - Embedded default config via `//go:embed permutations.yaml`
   - Loads patterns and payloads from YAML
   - Supports custom wordlist file references

5. **`replacer.go`** (~28 lines): Template variable replacement
   - Uses `projectdiscovery/fasttemplate` for fast substitution
   - Two-pass replacement with fallback marker `§`

6. **`cmd/alterx/main.go`** (~68 lines): CLI entry point
   - Bootstrap and flag parsing
   - Output coordination

7. **`internal/runner/runner.go`** (~140 lines): CLI flag parsing
   - Handles input sources (file, stdin, comma-separated)
   - Output options (file, stdout, size limiting)
   - Config loading

### Variable System

AlterX extracts these variables from input domains:

**Basic Variables:**
- `{{sub}}`: Leftmost subdomain prefix (e.g., "api" from "api.scanme.sh")
- `{{suffix}}`: Everything except {{sub}} (e.g., "scanme.sh")
- `{{tld}}`: Top-level domain (e.g., "sh")
- `{{etld}}`: Public suffix (e.g., "co.uk")

**Advanced Variables:**
- `{{sld}}`: Second-level domain (e.g., "scanme" from "api.scanme.sh")
- `{{root}}`: eTLD+1 / root domain (e.g., "scanme.sh")
- `{{subN}}`: Multi-level subdomain prefixes (e.g., `{{sub1}}`, `{{sub2}}`)

**Dynamic Payload Variables:**
- `{{word}}`: Custom wordlist (default: 60+ common subdomain words like "api", "dev", "prod")
- `{{number}}`: Numeric values
- `{{region}}`: Geographic identifiers
- Custom payloads can be added via config or CLI

### Pattern System

Patterns are templates describing permutation types. Examples:
```
{{word}}-{{sub}}.{{suffix}}     → prod-api.scanme.sh
{{sub}}-{{word}}.{{suffix}}     → api-prod.scanme.sh
{{word}}.{{sub}}.{{suffix}}     → prod.api.scanme.sh
{{sub}}{{number}}.{{suffix}}    → api01.scanme.sh
```

Default patterns are in `permutations.yaml` (embedded at compile time). The config is also written to `~/.config/alterx/permutation_v*.yaml` on first run for user customization.

### Data Flow

1. **Input Processing**: Parse domains into `Input` structs with extracted variables
2. **Enrichment** (optional): Extract words from input domains, add to `word` payload
3. **Pattern Iteration**: For each input × each pattern:
   - Validate pattern variables exist in input
   - Replace input-specific variables ({{sub}}, {{suffix}}, etc.)
   - Apply ClusterBomb algorithm for payload variables ({{word}}, {{number}}, etc.)
4. **Deduplication**: In-memory deduplication using `projectdiscovery/utils/dedupe`
5. **Output**: Stream to file or stdout with optional size limiting

## Important Patterns and Conventions

### Zero CGO Dependency
- Built with `CGO_ENABLED=0` for maximum portability
- Pure Go implementation with no C dependencies
- Docker images use Alpine Linux with minimal dependencies

### Streaming Architecture
- Results are streamed via Go channels for memory efficiency
- Supports large permutation sets without loading everything into memory
- Two execution modes: channel-based (`Execute()`) and writer-based (`ExecuteWithWriter()`)

### Embedded Configuration
- Default `permutations.yaml` is embedded using `//go:embed` directive
- No external file dependencies for default operation
- Config written to user home directory on first run for customization

### ProjectDiscovery Ecosystem Integration
- Uses ProjectDiscovery utilities (`goflags`, `gologger`, `utils`, `fasttemplate`)
- Compatible with other ProjectDiscovery tools (dnsx, subfinder, chaos)
- Follows ProjectDiscovery coding conventions and patterns

### Performance Optimizations
1. **Early exit in ClusterBomb**: Skips redundant word combinations
2. **Variable presence validation**: Filters patterns before processing
3. **Streaming results**: Channel-based output prevents memory bloat
4. **Payload filtering**: Only includes payloads referenced in active patterns
5. **Fast template engine**: Uses custom fasttemplate library

## Testing Strategy

- **Unit tests**: `mutator_test.go`, `inputs_test.go`
- **Integration tests**: GitHub Actions runs cross-platform builds (Linux, macOS, Windows)
- **Race detection**: CI runs tests with `-race` flag
- **Live testing**: Echo input through CLI with race detection

When adding tests, follow existing patterns in `*_test.go` files using `github.com/stretchr/testify`.

## Release and Distribution

### GoReleaser Configuration
- Builds for: Windows, Linux, macOS
- Architectures: amd64, 386, arm, arm64
- Format: ZIP archives with SHA256 checksums
- Announcements: Slack + Discord integration

### Docker
- Multi-stage build using Alpine Linux
- Build stage: `golang:1.21.6-alpine`
- Runtime stage: `alpine:3.19.0`
- Includes: ca-certificates, dig (bind-tools)

### CI/CD Workflows
- **build-test.yml**: Multi-OS builds on PR (Go 1.21.x)
- **lint-test.yml**: Code quality checks with golangci-lint
- **release-binary.yml**: Binary releases on tag push
- **dockerhub-push.yml**: Docker image push on release
- **codeql-analysis.yml**: Security scanning
- **dep-auto-merge.yml**: Automated dependency updates

## Current Branch: Pattern Induction Feature

**Branch**: `feat-language-induction`

This branch implements **automatic pattern learning** from passive subdomain enumeration results, based on the [regulator algorithm](https://github.com/cramppet/regulator) with significant optimizations for scalability.

### Pattern Induction Architecture

The pattern induction system learns subdomain naming conventions from observed data rather than requiring manual pattern specification. It follows a multi-phase approach:

#### Phase 1: Tokenization (✅ Implemented - `inducer/` package)

**Purpose**: Parse subdomains into structured token arrays preserving hierarchy and separators.

**Key Components**:
- **`inducer/tokenizer.go`**: Tokenization engine following regulator algorithm
  - Extracts subdomain using `publicsuffix` (removes root domain)
  - Splits by dots → DNS hierarchy levels (level 0, 1, 2...)
  - Splits each level by dashes → preserves dash positions
  - Splits by numbers → preserves numeric sequences
  - Special handling for hyphenated numbers (e.g., "-01" stays as one token)

- **`inducer/types.go`**: Core data structures
  - `Token`: Individual token with type (Word, Dash, Number) and position
  - `Level`: All tokens at a DNS hierarchy level
  - `TokenizedDomain`: Complete parsed subdomain with all levels
  - **API Convention**: 0-indexed internally, 1-indexed externally (GetLevel(1) = level 0)

- **`inducer/storage.go`**: Efficient indexing
  - `TokenIndex`: O(1) lookup - level → position → token → domain IDs
  - `LevelStats`: Per-level statistics and frequency analysis
  - Prepares for hierarchical partitioning (bounded group size: 5000)

- **`inducer/inducer.go`**: Public API
  - `NewPatternInducer()`: Creates inducer from domain list
  - `LoadAndTokenize()`: Processes all domains, builds indices
  - `GetLevel(n)`: 1-indexed external API for accessing levels
  - `Stats()`: Comprehensive statistics about tokenization results

**Tokenization Examples**:
```
"api-dev-01.staging.example.com" →
  Level 0: [Token{api, Word}, Token{-dev, Dash}, Token{-01, Dash}]
  Level 1: [Token{staging, Word}]

"db01.prod.internal.example.com" →
  Level 0: [Token{db, Word}, Token{01, Number}]
  Level 1: [Token{prod, Word}]
  Level 2: [Token{internal, Word}]
```

**Testing**:
- Comprehensive unit tests in `inducer/tokenizer_test.go`
- Example-based tests in `inducer/example_test.go`
- Run with: `go test ./inducer/`

#### Phase 2-5: Not Yet Implemented

The following phases are planned based on the [optimization strategy](./literature_survey/proposed_solution.md):

**Phase 2**: Edit distance clustering with hierarchical partitioning
- Build MEMO table only within bounded groups (≤5K domains)
- Apply multi-strategy clustering (global, n-gram, token-level)
- Achieve O(1) memory instead of regulator's O(N²)

**Phase 3**: Pattern generation and regex creation
- Convert token closures to regex patterns
- Compress number ranges (e.g., `01|02|03` → `[0-1][0-3]`)
- Quality filtering via ratio test

**Phase 4**: Integration with AlterX DSL
- Map learned patterns to AlterX template format
- Merge with manual patterns from `permutations.yaml`
- Support hybrid mode (manual + learned patterns)

**Phase 5**: CLI integration
- Add `-mode` flag: `default` (manual), `inferred` (learned), `both`
- Store learned patterns in `permutations.yaml` → `learned_patterns` section
- Confidence scoring and pattern ranking

### Pattern Induction Design Principles

Based on extensive [literature survey](./literature_survey/) and [regulator analysis](./literature_survey/regulator/):

1. **Hierarchical Prefix Partitioning**: Never build full N² MEMO table
   - Partition domains by prefix (1-gram, 2-gram, 3-gram)
   - Process groups ≤5K domains independently
   - Achieves **constant O(1) memory** (1-2 GB for any dataset size)

2. **Streaming Architecture**: Process groups in parallel via goroutines
   - Build MEMO → cluster → generate patterns → free MEMO
   - Each group is independent (no shared state)
   - Linear speedup with CPU cores

3. **Quality Over Quantity**: Ratio test filters overly broad patterns
   - Accept patterns with generation_count/observed_count < 25
   - Prevents pattern explosion while maintaining 80%+ precision

4. **Backward Compatibility**: Learned patterns use same DSL syntax
   - `{{service}}-{{env}}-{{number}}.{{suffix}}`
   - Can be merged with manual patterns
   - No breaking changes to existing functionality

### Pattern Induction Configuration

The `permutations.yaml` file has been extended with pattern induction sections:

```yaml
## BACKWARD COMPATIBLE - EXISTING FORMAT
patterns:
  - "{{word}}-{{sub}}.{{suffix}}"
  # ... existing manual patterns

payloads:
  word: [api, dev, prod]
  # ... existing payloads

## PATTERN INDUCTION EXTENSIONS
token_dictionary:
  # Semantic token classifications for auto-classification
  env: [dev, prod, staging, qa]
  region: [us-east-1, us-west-2, eu-central-1]
  service: [api, web, cdn, db]

learned_patterns:
  # Auto-generated during pattern induction
  - id: pattern_001
    template: "{{service}}-{{env}}-{{number}}.{{suffix}}"
    coverage: 450      # Domains matched
    ratio: 1.2         # Generations/observed
    confidence: 0.84   # Quality score (0-1)
```

See [`literature_survey/config_format.md`](./literature_survey/config_format.md) for complete specification.

### Development Workflow for Pattern Induction

When working on pattern induction features:

1. **Read the literature survey first**: [`literature_survey/README.md`](./literature_survey/README.md)
   - Understand why regulator fails to scale (O(N²) space)
   - Learn the hierarchical partitioning solution
   - Review the optimization strategy

2. **Study the regulator algorithm**: [`literature_survey/regulator/algorithm.md`](./literature_survey/regulator/algorithm.md)
   - Tokenization rules (preserve dashes, split numbers)
   - Edit distance clustering (multi-strategy approach)
   - Pattern generation (closure_to_regex)
   - Quality filtering (ratio test)

3. **Current implementation status**:
   - ✅ Phase 1: Tokenization complete in `inducer/` package
   - ⏳ Phase 2-5: Not yet implemented

4. **Integration points**:
   - `Options.Domains` in `internal/runner/runner.go` provides input domains
   - `Options.Mode` flag controls pattern mode (default/inferred/both)
   - `Config` in `config.go` loads `permutations.yaml`
   - Pattern learning populates `learned_patterns` section

## Key Dependencies

- **goflags**: CLI flag parsing (ProjectDiscovery)
- **gologger**: Structured logging (ProjectDiscovery)
- **fasttemplate**: Fast template processing (ProjectDiscovery)
- **utils**: File, URL, dedup utilities (ProjectDiscovery)
- **golang.org/x/net/publicsuffix**: Accurate public suffix detection
- **gopkg.in/yaml.v3**: YAML configuration parsing

## Code Modification Guidelines

1. **Maintain streaming architecture**: Don't load all results into memory
2. **Preserve zero-CGO**: No C dependencies
3. **Follow ProjectDiscovery patterns**: Use their utility libraries
4. **Embedded config**: Keep default config embedded for portability
5. **Backward compatibility**: Don't break existing pattern syntax
6. **Performance-first**: This tool processes millions of permutations
7. **Security context**: Remember this is a security research tool, handle inputs safely
