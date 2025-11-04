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
# Build binary
make build              # Output: bin/alterx
go build -o alterx cmd/alterx/main.go

# Install to GOPATH/bin
make install
go install github.com/projectdiscovery/alterx/cmd/alterx@latest

# Run directly from source
go run cmd/alterx/main.go -l domains.txt

# Run examples
go run examples/basic/main.go
go run examples/pattern_induction/main.go
```

### Testing
```bash
# Run all tests
make test
go test ./...

# Run tests with race detection
make test-race
go test -race ./...

# Run specific test
go test -run TestMutator
go test -v ./internal/inducer/... -run TestOrchestrator

# Test pattern induction components
make test-inducer
go test ./internal/inducer/...

# Coverage report
make test-cover        # Generates coverage.html
```

### Linting and Formatting
```bash
# Format code
make fmt
go fmt ./...

# Run linter (requires golangci-lint)
make lint
golangci-lint run --timeout=5m

# Run go vet
make vet
go vet ./...

# All verification steps
make verify            # fmt + vet + lint + test
```

### Pattern Induction Evaluation
```bash
# Run evaluation framework (separate subproject)
cd eval
make run               # Full evaluation on 8 benchmark domains
make validate          # Quick validation on 2 domains
make show              # View results summary
make diff-baseline     # Compare with saved baseline
```

## Architecture Overview

### Core Components

The codebase follows a **template-based permutation engine** architecture with **learned pattern support**:

```
Input Domains → Parse Variables → Apply Patterns → ClusterBomb Algorithm → Deduplicate → Output
                      ↓
              Pattern Induction (optional)
```

**Key files and their responsibilities:**

1. **`mutator.go`** (~500 lines): Core permutation engine
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

4. **`config.go`** (~120 lines): Configuration file handling
   - Embedded default config via `//go:embed permutations.yaml`
   - Loads patterns and payloads from YAML
   - Supports custom wordlist file references
   - New: Token dictionary support for semantic classification

5. **`induction.go`** (~224 lines): Pattern induction public API
   - `PatternInducer`: Learns patterns from passive subdomain enumeration
   - `InferPatterns()`: Returns `LearnedPattern` objects with metadata
   - Integration with orchestrator and filtering pipeline

6. **`replacer.go`** (~28 lines): Template variable replacement
   - Uses `projectdiscovery/fasttemplate` for fast substitution
   - Two-pass replacement with fallback marker `§`

7. **`cmd/alterx/main.go`** (~68 lines): CLI entry point
   - Bootstrap and flag parsing
   - Output coordination

8. **`internal/runner/runner.go`** (~140 lines): CLI flag parsing
   - Handles input sources (file, stdin, comma-separated)
   - Output options (file, stdout, size limiting)
   - Config loading

### Pattern Induction System

**Location**: `internal/inducer/` (40 Go files, ~4000+ lines)

The pattern induction system is **fully implemented** and learns subdomain naming conventions from observed data. It follows a multi-phase approach:

#### Core Components

**1. Orchestrator** (`orchestrator.go` ~1115 lines)
- Central coordinator for pattern induction pipeline
- Implements 3 clustering strategies from regulator algorithm
- Level-based grouping to respect structural boundaries
- Entropy-based pattern budget for intelligent pruning
- Affinity Propagation auto-tuning

**2. DSL Generator** (`dsl_generator.go`, `dsl_converter.go`)
- Converts regex patterns to AlterX DSL templates
- Semantic token classification using dictionary
- Number range compression (e.g., `01-99` → `NumberRange`)
- Variable naming with semantic awareness

**3. Edit Distance Clustering** (`editdistance.go`, `distance.go`)
- Memoized edit distance calculation (O(N²) space)
- Three clustering strategies:
  - Strategy 1: Global clustering
  - Strategy 2: N-gram prefix anchoring
  - Strategy 3: Token-level clustering

**4. Pattern Quality Filtering** (`filter.go`, `validation.go`)
- Ratio test: filters overly broad patterns
- Confidence scoring based on structural diversity
- Adaptive thresholds based on dataset size

**5. Pattern Clustering** (`clustering.go`, `affinity_propagation.go`)
- Affinity Propagation for pattern consolidation
- Pure structural similarity (template + token sequence)
- Auto-tuning to target specific pattern counts

**6. Pattern Budget** (`pattern_budget.go`, `autotuner.go`)
- Entropy-based selection using structural diversity
- Coverage efficiency analysis with elbow detection
- Prevents pattern explosion while maximizing coverage

**7. Level Grouping** (`level_grouping.go`)
- Groups domains by structural depth (level count)
- Prevents mixing different hierarchical structures
- Independent MEMO tables per group for memory efficiency

**8. Enrichment** (`enricher.go`)
- Adds optional variable support ("" marker)
- Enables ClusterBomb flexibility for pattern matching

**9. Supporting Components**
- `trie.go`: Prefix-based domain indexing
- `compression.go`: Number range optimization
- `types.go`: Core data structures (Token, Closure, DSLPattern)
- `patterns.go`: Pattern metadata and quality checks

#### Algorithm Overview

Based on the [regulator algorithm](https://github.com/cramppet/regulator) with optimizations:

```
1. Level-based Grouping
   ↓
2. Per-level Processing:
   - Build local MEMO table (edit distances)
   - Build Trie (prefix indexing)
   - Strategy 1: Global clustering (all domains)
   - Strategy 2: N-gram prefix clustering (2-3 gram groups)
   - Strategy 3: Token-level clustering (first token groups)
   ↓
3. Pattern Generation (DSL direct, no regex intermediate)
   ↓
4. Quality Filtering (ratio test, confidence scoring)
   ↓
5. Pattern Consolidation (Affinity Propagation clustering)
   ↓
6. Entropy-based Budget (intelligent pruning)
   ↓
7. Enrichment (optional variables)
```

**Key Optimizations**:
- Level-based grouping prevents mixing incompatible structures
- Local MEMO tables per group (bounded memory)
- Entropy-based budget prevents pattern explosion
- Direct DSL generation (bypasses regex → DSL conversion)
- Affinity Propagation consolidates structurally similar patterns

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

**Pattern Induction Variables:**
- `{{p0}}`, `{{p1}}`, etc.: Positional payloads (literals)
- `{{n0}}`, `{{n1}}`, etc.: Number ranges
- Semantic variables: `{{env}}`, `{{region}}`, `{{service}}` (when dictionary provided)

### Pattern System

Patterns are templates describing permutation types. Examples:

**Manual Patterns** (defined in `permutations.yaml`):
```
{{word}}-{{sub}}.{{suffix}}     → prod-api.scanme.sh
{{sub}}-{{word}}.{{suffix}}     → api-prod.scanme.sh
{{word}}.{{sub}}.{{suffix}}     → prod.api.scanme.sh
{{sub}}{{number}}.{{suffix}}    → api01.scanme.sh
```

**Learned Patterns** (from pattern induction):
```
{{p0}}-{{p1}}.{{root}}          → api-dev.example.com (p0: [api,web], p1: [dev,prod])
{{env}}-{{n0}}.{{root}}         → staging-01.example.com (env: [dev,staging], n0: 0-99)
```

Default patterns are in `permutations.yaml` (embedded at compile time). The config is also written to `~/.config/alterx/permutation_v*.yaml` on first run for user customization.

### Data Flow

**Standard Mode**:
1. **Input Processing**: Parse domains into `Input` structs with extracted variables
2. **Enrichment** (optional): Extract words from input domains, add to `word` payload
3. **Pattern Iteration**: For each input × each pattern:
   - Validate pattern variables exist in input
   - Replace input-specific variables ({{sub}}, {{suffix}}, etc.)
   - Apply ClusterBomb algorithm for payload variables ({{word}}, {{number}}, etc.)
4. **Deduplication**: In-memory deduplication using `projectdiscovery/utils/dedupe`
5. **Output**: Stream to file or stdout with optional size limiting

**Pattern Induction Mode** (programmatic API):
1. **Input**: Passive subdomain enumeration results
2. **Filtering**: Remove wildcards, root-only domains, invalid TLDs
3. **Level Grouping**: Group by structural depth (subdomain levels)
4. **Per-level Processing**:
   - Build local MEMO table
   - Apply 3 clustering strategies
   - Generate DSL patterns directly
5. **Quality Filtering**: Ratio test, confidence scoring
6. **Pattern Consolidation**: Affinity Propagation clustering
7. **Entropy Budget**: Intelligent pruning to target pattern count
8. **Enrichment**: Add optional variable support
9. **Output**: `LearnedPattern` objects with templates, payloads, metadata

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
6. **Level-based grouping**: Bounded MEMO tables per structural group
7. **Entropy-based budget**: Intelligent pattern pruning

### Pattern Induction Performance
- **Memory**: Bounded per level group (typically 500MB - 2GB peak)
- **Time**: 1-5 minutes for 100-1000 domains, 10-30 minutes for 5000+ domains
- **Scalability**: Level-based grouping prevents O(N²) memory explosion
- **Optimization**: Adaptive strategies skip expensive operations for large groups

## Testing Strategy

- **Unit tests**: `*_test.go` files in root and `internal/inducer/`
- **Integration tests**: GitHub Actions runs cross-platform builds (Linux, macOS, Windows)
- **Race detection**: CI runs tests with `-race` flag
- **Benchmarks**: `*_bench_test.go` files for performance tracking
- **Evaluation framework**: `eval/` subproject for pattern induction quality validation

When adding tests, follow existing patterns using `github.com/stretchr/testify`.

## Pattern Induction Configuration

The `permutations.yaml` file supports pattern induction extensions:

```yaml
## BACKWARD COMPATIBLE - EXISTING FORMAT
patterns:
  - "{{word}}-{{sub}}.{{suffix}}"
  # ... existing manual patterns

payloads:
  word: [api, dev, prod]
  # ... existing payloads

## PATTERN INDUCTION EXTENSIONS (programmatic API only, not CLI)
token_dictionary:
  # Semantic token classifications for auto-classification
  env: [dev, prod, staging, qa]
  region: [us-east-1, us-west-2, eu-central-1]
  service: [api, web, cdn, db]

# Note: learned_patterns section exists in code but is programmatic only
# CLI does not currently expose pattern induction (future work)
```

See [`literature_survey/config_format.md`](./literature_survey/config_format.md) for complete specification.

## Current Branch: Pattern Induction Feature

**Branch**: `feat-language-induction`

This branch implements **automatic pattern learning** from passive subdomain enumeration results, based on the [regulator algorithm](https://github.com/cramppet/regulator) with significant optimizations for scalability.

### Implementation Status: ✅ FULLY IMPLEMENTED

**All phases complete** (contrary to earlier documentation):
- ✅ Phase 1: Tokenization and indexing
- ✅ Phase 2: Edit distance clustering with level-based grouping
- ✅ Phase 3: Pattern generation and DSL conversion
- ✅ Phase 4: Quality filtering and consolidation
- ✅ Phase 5: Entropy-based budget and enrichment

**Additional features implemented**:
- ✅ Affinity Propagation clustering for pattern consolidation
- ✅ Auto-tuning for optimal cluster counts
- ✅ Entropy-based pattern budget with coverage analysis
- ✅ Evaluation framework for quality validation

### Programmatic API Usage

Pattern induction is available programmatically (not yet exposed in CLI):

```go
// Example: Learn patterns from passive enumeration
import "github.com/projectdiscovery/alterx"

passiveDomains := []string{
    "api-dev.example.com",
    "api-prod.example.com",
    "web-staging.example.com",
    // ... more domains
}

// Create pattern inducer
inducer := alterx.NewPatternInducer(passiveDomains, 2)

// Learn patterns
patterns, err := inducer.InferPatterns()
if err != nil {
    // Handle error
}

// Use learned patterns
for _, pattern := range patterns {
    fmt.Printf("Template: %s\n", pattern.Template)
    fmt.Printf("Coverage: %d domains\n", pattern.Coverage)
    fmt.Printf("Confidence: %.2f\n", pattern.Confidence)
    // pattern.Payloads contains inline payloads
}
```

### Development Workflow for Pattern Induction

When working on pattern induction features:

1. **Read the literature survey**: [`literature_survey/README.md`](./literature_survey/README.md)
   - Understand regulator algorithm and optimizations
   - Review level-based grouping strategy
   - See performance analysis

2. **Study the implementation**:
   - `induction.go`: Public API
   - `internal/inducer/orchestrator.go`: Pipeline coordinator
   - `internal/inducer/dsl_generator.go`: Pattern generation
   - `internal/inducer/clustering.go`: Pattern consolidation
   - `internal/inducer/pattern_budget.go`: Entropy-based selection

3. **Test changes with evaluation framework**:
   ```bash
   cd eval
   make run                    # Full evaluation
   make save-baseline          # Save baseline
   # Make your changes
   make run                    # Re-evaluate
   make diff-baseline          # Compare
   ```

4. **Key metrics to monitor**:
   - `avg_f1_score`: Overall pattern quality (target: >0.70)
   - `avg_coverage`: Percentage of domains matched
   - `peak_memory_mb`: Memory usage
   - `execution_time_seconds`: Performance

### Integration Points

- `induction.go`: Public API (`NewPatternInducer`, `InferPatterns`)
- `config.go`: Token dictionary loading (`GetTokenDictionary()`)
- `mutator.go`: Pattern application (uses same `Mutator` engine)
- `internal/runner/runner.go`: CLI integration point (future work)

## Evaluation Framework

**Location**: `eval/` subdirectory (separate Go module)

A comprehensive testing system for validating pattern induction quality. Tests against 8 diverse benchmark domains (google.com, tesla.com, netflix.com, etc.) from ProjectDiscovery Chaos dataset.

**Key Commands**:
```bash
cd eval
make run          # Full evaluation (10-30 minutes)
make validate     # Quick validation (2 domains, 3-5 minutes)
make show         # View results summary
make diff-baseline # Compare with saved baseline
```

**Output**: Single comprehensive file `eval/results/evaluation_results.json` with:
- Summary metrics (avg F1, coverage, precision, recall)
- Per-domain detailed analysis
- Pattern quality scores
- Performance metrics

See [`eval/README.md`](./eval/README.md) and [`eval/CLAUDE.md`](./eval/CLAUDE.md) for complete documentation.

## Key Dependencies

- **goflags**: CLI flag parsing (ProjectDiscovery)
- **gologger**: Structured logging (ProjectDiscovery)
- **fasttemplate**: Fast template processing (ProjectDiscovery)
- **utils**: File, URL, dedup utilities (ProjectDiscovery)
- **golang.org/x/net/publicsuffix**: Accurate public suffix detection
- **gopkg.in/yaml.v3**: YAML configuration parsing
- **github.com/agnivade/levenshtein**: Edit distance calculation

## Code Modification Guidelines

1. **Maintain streaming architecture**: Don't load all results into memory
2. **Preserve zero-CGO**: No C dependencies
3. **Follow ProjectDiscovery patterns**: Use their utility libraries
4. **Embedded config**: Keep default config embedded for portability
5. **Backward compatibility**: Don't break existing pattern syntax
6. **Performance-first**: This tool processes millions of permutations
7. **Security context**: Remember this is a security research tool, handle inputs safely
8. **Test with eval framework**: Validate pattern induction changes with `eval/make run`

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
