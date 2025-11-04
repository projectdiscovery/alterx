# Pattern Induction - Technical Deep Dive

> **Reading Time:** 10-15 minutes
> **Audience:** Red teamers, bug bounty hunters, security researchers
> **Prerequisites:** Basic understanding of subdomain enumeration

## Table of Contents
- [Introduction](#introduction)
- [Credits](#credits)
- [The Problem We're Solving](#the-problem-were-solving)
- [Execution Modes](#execution-modes)
- [The Pipeline](#the-pipeline)
  - [Step 1: Input Filtering](#step-1-input-filtering)
  - [Step 2: Mode Detection](#step-2-mode-detection)
  - [Step 3: Level-Based Grouping](#step-3-level-based-grouping)
  - [Step 4: Adaptive Group Sampling](#step-4-adaptive-group-sampling-fast-mode-only)
  - [Step 5: Building Local Indexes](#step-5-building-local-indexes)
  - [Step 6: Three-Stage Clustering](#step-6-three-stage-clustering)
  - [Step 7: DSL Pattern Conversion](#step-7-dsl-pattern-conversion)
  - [Step 8: Deduplication](#step-8-deduplication)
  - [Step 9: Affinity Propagation](#step-9-affinity-propagation-consolidation)
  - [Step 10: Entropy-Based Selection](#step-10-entropy-based-selection)
  - [Step 11: Enrichment](#step-11-enrichment)
- [Practical Examples](#practical-examples)
- [Performance Considerations](#performance-considerations)

---

## Introduction

**Pattern induction** is AlterX's ability to automatically learn subdomain naming patterns from observed data. Instead of manually guessing what patterns a target organization uses (like `api-{env}.example.com` or `{service}-v{number}.example.com`), pattern induction analyzes your passive enumeration results and discovers these patterns for you.

**Why this matters for red teamers:**
- Organizations have unique naming conventions that generic wordlists miss
- Learning target-specific patterns dramatically reduces noise in your wordlists
- Automated pattern discovery scales better than manual pattern writing
- Generates focused, high-probability subdomain candidates for active enumeration

---

## Credits

AlterX's pattern induction is inspired by the [**Regulator**](https://github.com/cramppet/regulator) algorithm by [@cramppet](https://github.com/cramppet)

**What we adapted from Regulator:**
- Core concept: Edit distance-based clustering for pattern discovery
- Hierarchical partitioning strategy
- Quality filtering based on generativity ratio

**What we optimized for subdomain enumeration:**
- Level-based grouping to handle multi-level subdomains efficiently
- Three adaptive modes (THOROUGH/BALANCED/FAST) based on dataset size
- Direct DSL generation (bypassing regex intermediate representation)
- Affinity Propagation for pattern consolidation
- Entropy-based budget system to prevent pattern explosion
- Parallelization at multiple levels for real-world scalability

---

## The Problem We're Solving

### Traditional Approach Problem
When you run passive enumeration on `example.com`, you might discover:
```
api-dev.example.com
api-prod.example.com
api-staging.example.com
web-dev.example.com
web-prod.example.com
cdn-prod.example.com
```

**Manual approach:** You'd have to:
1. Manually notice the `{service}-{env}` pattern
2. Write patterns in AlterX DSL: `{{p0}}-{{p1}}.{{root}}`
3. Define payloads: `p0: [api, web, cdn]`, `p1: [dev, prod, staging]`
4. Hope you didn't miss any patterns or values

**Pattern induction approach:** Feed all 6 domains to AlterX, and it automatically:
1. Discovers the `{service}-{env}` pattern
2. Extracts all observed values
3. Generates DSL templates with payloads
4. Filters out low-quality patterns
5. Ready to generate permutations for active scanning

### Scale Challenge
- **Small targets:** 50-100 subdomains (manually feasible but tedious)
- **Medium targets:** 500-1000 subdomains (manual analysis impractical)
- **Large targets:** 5000+ subdomains (impossible to analyze manually)

Pattern induction scales from small to large datasets with adaptive optimization.

---

## Execution Modes

AlterX automatically selects one of three modes based on your input size. Each mode trades off accuracy vs. performance.

### Mode Selection Table

| Mode | Dataset Size | Target Coverage | Pattern Range | Use Case |
|------|-------------|----------------|---------------|----------|
| **THOROUGH** | <100 domains | 95% | 8-30 patterns | Small targets, highest accuracy |
| **BALANCED** | 100-1000 domains | 90% | 5-25 patterns | Most common scenarios |
| **FAST** | >1000 domains | 85% | 3-20 patterns | Large datasets, performance-critical |

### Parameter Matrix

| Parameter | THOROUGH | BALANCED | FAST | Purpose |
|-----------|----------|----------|------|---------|
| **Target Coverage** | 95% | 90% | 85% | Minimum % of input domains patterns should match |
| **Elbow Sensitivity** | 0.01 | 0.02 | 0.03 | How aggressively to stop adding patterns (higher = fewer patterns) |
| **Min Patterns** | 8 | 5 | 3 | Minimum patterns to generate |
| **Max Patterns** | 30 | 25 | 20 | Maximum patterns to generate |
| **Distance Range** | 2-8 | 2-6 | 2-4 | Edit distance thresholds for clustering |
| **Max Ratio** | 18.0 | 15.0 | 12.0 | Filter patterns that generate too many permutations |
| **AP Iterations** | 12 | 10 | 6 | Affinity Propagation clustering iterations |
| **Enrichment Rate** | 80% | 50% | 50% | % of patterns to make variables optional |
| **N-gram Strategy** | Disabled | Enabled (>200) | Enabled (>100) | Enable prefix-based clustering strategy |
| **Token Limiting** | Disabled | Disabled | Enabled (30 groups) | Limit token groups in Strategy 3 |
| **Group Sampling** | Disabled | Disabled | Enabled (500 max) | Sample large level groups |

**Why these differences?**
- **THOROUGH:** Small datasets allow exhaustive analysis without performance penalties
- **BALANCED:** Moderate optimizations while maintaining good accuracy
- **FAST:** Aggressive optimizations for large datasets where exhaustive analysis would take hours

---

## The Pipeline

Pattern induction runs through 11 steps. Let's walk through each one.

### Step 1: Input Filtering

**What we do:**
Remove subdomains that don't contribute meaningful patterns:
- Wildcard domains (`*.example.com`)
- Root-only domains (`example.com`)
- Invalid TLDs or malformed domains

**Why:**
These domains introduce noise and don't represent actual subdomain patterns. For example, wildcards are DNS configurations, not real subdomains.

**Regulator comparison:**
Regulator operates on generic strings. We added domain-specific filtering because subdomain data has unique characteristics (TLDs, wildcards) that need special handling.

**Example:**
```
Input: api.example.com, *.cdn.example.com, example.com, api-dev.example.com
Output: api.example.com, api-dev.example.com
```

---

### Step 2: Mode Detection

**What we do:**
Count filtered domains and select execution mode (THOROUGH/BALANCED/FAST).

**Why:**
Different dataset sizes need different optimization strategies. Small datasets can afford thorough analysis; large datasets need aggressive optimizations to finish in reasonable time.

**Regulator comparison:**
Regulator has no adaptive modes—it runs the same algorithm regardless of input size. We added this because subdomain datasets vary drastically (from 50 to 50,000+ domains).

**Example:**
```
50 domains → THOROUGH mode
500 domains → BALANCED mode
5000 domains → FAST mode
```

---

### Step 3: Level-Based Grouping

**What we do:**
Group domains by their **structural depth** (number of subdomain levels).

**Level examples:**
- Level 2: `api.example.com` (1 subdomain level)
- Level 3: `api-v2.prod.example.com` (2 subdomain levels)
- Level 4: `api.east.us.example.com` (3 subdomain levels)

**Why:**
Domains with different structural depths rarely share patterns. Grouping them:
- **Prevents pattern pollution:** A 2-level pattern won't match 4-level domains
- **Improves memory efficiency:** Smaller groups = smaller edit distance tables
- **Enables parallelization:** Each group processes independently

**Regulator comparison:**
Regulator doesn't have level-based grouping—it treats all strings uniformly. This works for generic regex synthesis but causes memory explosion on large subdomain datasets because the edit distance table grows O(N²).

Our approach: By splitting into level groups, we get multiple smaller O(M²) tables where M << N, bounded memory per group.

**Example:**
```
Input: api.example.com, web.example.com, api-v2.prod.example.com

Level 2 Group: [api.example.com, web.example.com]
Level 3 Group: [api-v2.prod.example.com]
```

**Performance impact:**
- Without grouping: 10,000 domains = 100M distance calculations
- With grouping: 5 groups of 2,000 domains each = 5×4M = 20M calculations (5x speedup)

---

### Step 4: Adaptive Group Sampling (FAST Mode Only)

**What we do:**
For level groups larger than 500 domains (FAST mode), intelligently sample a subset using **stratified sampling**.

**How stratified sampling works:**
1. Partition domains by their first token (e.g., `api-*`, `web-*`, `cdn-*`)
2. Calculate frequency of each partition
3. Apply sampling rules:
   - **Rare tokens** (< 5% frequency): Keep all (important edge cases)
   - **Common tokens** (5-50% frequency): Keep 60% (representative sample)
   - **Dominant tokens** (> 50% frequency): Keep 200 max (prevent skew)

**Why:**
Large groups (>500 domains) create massive edit distance tables (>250K calculations). Sampling reduces this while preserving diversity.

**Regulator comparison:**
Regulator has no sampling—it always processes the full dataset. For large inputs, this becomes prohibitively slow (hours vs. minutes).

**Example:**
```
Input group (1000 domains):
- api-* prefix: 600 domains (60% dominant) → Sample 200
- web-* prefix: 300 domains (30% common) → Keep 180 (60%)
- cdn-* prefix: 100 domains (10% rare) → Keep all 100

Output: 480 sampled domains (preserves diversity, reduces computation)
```

**Trade-off:**
Sampling may miss rare patterns in dominant token groups, but maintains 85% target coverage (FAST mode goal).

---

### Step 5: Building Local Indexes

**What we do:**
For each level group, build two data structures:
1. **MEMO Table:** Precomputed edit distances between all domain pairs
2. **Trie:** Prefix tree for fast domain lookup

**Why each index:**

**MEMO Table (Edit Distance Cache):**
- Edit distance is expensive to compute (O(N×M) for two strings)
- We'll need to compute distances thousands of times during clustering
- Precompute once, lookup in O(1) during clustering

**Trie (Prefix Tree):**
- Strategy 2 (N-gram clustering) needs fast prefix lookups
- Strategy 3 (token-level clustering) groups by first token
- Trie enables O(k) prefix search vs. O(N) linear scan

**Regulator comparison:**
Regulator uses memoization for edit distances but doesn't use Tries. We added Tries because subdomain structure makes prefix-based operations common (e.g., finding all `api-*` domains).

**Example:**
```
Domains: api-dev.example.com, api-prod.example.com, web-dev.example.com

MEMO Table:
- distance(api-dev, api-prod) = 4 (dev→prod)
- distance(api-dev, web-dev) = 3 (api→web)
- distance(api-prod, web-dev) = 5

Trie:
api-
  ├─ dev
  └─ prod
web-
  └─ dev
```

**Performance impact:**
- Without MEMO: Recalculate edit distance every time (1000x slower)
- Without Trie: Linear scan for prefix searches (10-100x slower for Strategy 2)

---

### Step 6: Three-Stage Clustering

**What we do:**
Run three different clustering strategies **in parallel** to discover patterns:

1. **Strategy 1: Global Clustering** - Clusters all domains by edit distance
2. **Strategy 2: N-gram Prefix Anchoring** - Clusters domains with similar prefixes
3. **Strategy 3: Token-Level Clustering** - Clusters domains starting with the same token

**Why three strategies:**
Different patterns are best discovered by different approaches. Running all three and merging results maximizes coverage.

---

#### Strategy 1: Global Clustering

**How it works:**
1. For each edit distance threshold (2, 4, 6, 8 in THOROUGH mode):
   - Group domains within that distance of each other
   - Each group = one potential pattern

**Why:**
Discovers patterns where domains are structurally similar overall.

**Regulator comparison:**
This is similar to Regulator's main clustering approach. We iterate over multiple distance thresholds (Regulator uses a single threshold).

**Example:**
```
Distance threshold = 3:
Cluster: [api-dev.example.com, api-prod.example.com, web-dev.example.com]
Pattern: {p0}-{p1}.example.com where p0=[api,web], p1=[dev,prod]
```

**Good for:** Simple substitution patterns like `{service}-{env}`

---

#### Strategy 2: N-gram Prefix Anchoring (Conditional)

**When enabled:** BALANCED and FAST modes, if group size > threshold (200 for BALANCED, 100 for FAST)

**How it works:**
1. Partition domains by 2-gram or 3-gram prefixes:
   - 2-gram: First 2 tokens (e.g., `api-dev-*`)
   - 3-gram: First 3 tokens (e.g., `api-dev-v2-*`)
2. Within each partition, run edit distance clustering
3. Each partition generates independent patterns

**Why:**
Large datasets have subgroups with common prefixes. Partitioning:
- Reduces comparison space (cluster within partition, not globally)
- Discovers prefix-anchored patterns more efficiently
- Prevents rare prefix patterns from being lost in noise

**Regulator comparison:**
Regulator doesn't do prefix partitioning. We added this optimization because subdomain datasets often have strong prefix structure (e.g., all `api-*` subdomains follow one pattern, all `cdn-*` follow another).

**Example:**
```
Input: api-dev-v1.example.com, api-dev-v2.example.com, cdn-us-east.example.com

2-gram partitions:
- api-dev: [api-dev-v1, api-dev-v2] → Pattern: api-dev-{version}
- cdn-us: [cdn-us-east] → Pattern: cdn-us-east

Without partitioning: Might miss api-dev pattern if cdn-* patterns dominate
```

**Good for:** Prefix-stable patterns like `api-{region}-{env}` where the first part is always the same

---

#### Strategy 3: Token-Level Clustering

**How it works:**
1. Partition domains by first token (e.g., `api`, `web`, `cdn`)
2. Within each token group, run edit distance clustering
3. If FAST mode + >30 token groups: Keep only the 30 largest groups

**Why:**
Organizations often namespace subdomains by service (all `api-*` follow one pattern, all `web-*` follow another). Token-level partitioning:
- Isolates service-specific patterns
- Prevents cross-service pattern pollution
- Handles very large datasets by focusing on major tokens

**Regulator comparison:**
Regulator doesn't partition by tokens. We added this because subdomains have strong first-token semantics (it's often the service name).

**Example:**
```
Input: api-dev.example.com, api-prod.example.com, web-staging.example.com

Token partitions:
- api: [api-dev, api-prod] → Pattern: api-{env}
- web: [web-staging] → Pattern: web-{env}
```

**Good for:** Service-namespaced patterns where the first token identifies the service type

---

#### Strategy Parallelization

**What we do:**
All three strategies run simultaneously in parallel goroutines.

**Why:**
Strategies are independent—no data dependencies. Running in parallel:
- 3x speedup on multi-core systems
- Critical for FAST mode on large datasets

**Regulator comparison:**
Regulator is sequential. We parallelized because modern systems have multi-core CPUs and subdomain datasets are embarrassingly parallel.

---

### Step 7: DSL Pattern Conversion

**What we do:**
Convert each cluster of similar domains into an AlterX DSL template with payloads.

**Process:**
1. **Tokenize:** Split domains into tokens (e.g., `api-dev.example.com` → `[api, dev, example, com]`)
2. **Align:** Find common structure across cluster domains
3. **Identify variables:** Positions where tokens differ → variables
4. **Identify literals:** Positions where tokens are identical → literals
5. **Classify tokens:** Use token dictionary to assign semantic names (env, service, region) or default to positional (p0, p1)
6. **Generate DSL:** Build template with placeholders and extract payload values

**Why:**
Raw clusters are just groups of similar strings. DSL templates are actionable—you can use them to generate new subdomains.

**Regulator comparison:**
Regulator generates regex patterns, which must be converted to DSL. We generate DSL directly because:
- Regex → DSL conversion is lossy and error-prone
- DSL is AlterX's native format
- Direct generation is faster and more accurate

**Example:**
```
Cluster: [api-dev.example.com, api-prod.example.com, web-dev.example.com]

Tokenization:
- api-dev.example.com → [api, dev]
- api-prod.example.com → [api, prod]
- web-dev.example.com → [web, dev]

Alignment:
Position 0: [api, api, web] → Variable (p0)
Position 1: [dev, prod, dev] → Variable (p1)

DSL Template: {{p0}}-{{p1}}.{{root}}
Payloads:
  p0: [api, web]
  p1: [dev, prod]
```

**Number range optimization:**
If a variable contains numbers (e.g., `v1`, `v2`, `v3`), we infer a range:
```
Numbers: [01, 02, 03]
→ Range: 00-04 (min-1 to max+1, or min-2 if min-1 < 0)
→ DSL: {{n0}} with NumberRange{Min:0, Max:4}
```

---

### Step 8: Deduplication

**What we do:**
Remove duplicate patterns that emerged from different strategies.

**Why:**
Multiple strategies can discover the same pattern. For example:
- Strategy 1 (global) might find `api-{env}` pattern
- Strategy 3 (token-level) might also find `api-{env}` pattern from the `api` token group

Keeping duplicates wastes computational resources and generates redundant permutations.

**How we detect duplicates:**
Compare normalized template structure:
- Template string
- Variable positions
- Payload sets (order-independent)

**Regulator comparison:**
Regulator deduplicates regex patterns. We deduplicate DSL templates with payload-aware comparison.

**Example:**
```
Before deduplication:
1. {{p0}}-{{p1}}.{{root}} | p0=[api,web], p1=[dev,prod] (from Strategy 1)
2. {{p0}}-{{p1}}.{{root}} | p0=[api,web], p1=[dev,prod] (from Strategy 3)

After deduplication:
1. {{p0}}-{{p1}}.{{root}} | p0=[api,web], p1=[dev,prod]
```

---

### Step 9: Affinity Propagation (Consolidation)

**When enabled:** If pattern count > MaxPatterns (30 for THOROUGH, 25 for BALANCED, 20 for FAST)

**What we do:**
Use Affinity Propagation clustering to consolidate structurally similar patterns into representatives.

**How Affinity Propagation works (simplified):**
Imagine you have 50 patterns, but many are variations of the same structure:
- `api-{env}.example.com`
- `api-{env}-{region}.example.com`
- `web-{env}.example.com`

Affinity Propagation:
1. Calculates structural similarity between all pattern pairs
2. Let patterns "vote" for which one should represent their group
3. Emerges with ~20 exemplar patterns that best represent the whole set

**Why:**
Too many patterns:
- Slow down permutation generation
- Generate overlapping subdomains
- Create cognitive overload for manual review

Consolidation maintains coverage while reducing redundancy.

**Regulator comparison:**
Regulator doesn't have pattern consolidation. We added this because subdomain patterns often have minor variations that don't justify separate patterns.

**Similarity metric:**
We use pure structural similarity:
- Compare template structure (number of variables, positions)
- Compare token sequences (literals between variables)
- Ignore payload content (structural focus)

**Example:**
```
Before AP (35 patterns):
1. {{p0}}-{{p1}}.{{root}} | Coverage: 50
2. {{p0}}-{{p1}}-{{p2}}.{{root}} | Coverage: 30
3. {{p0}}.{{p1}}.{{root}} | Coverage: 40
... (32 more patterns with coverage 2-5 each)

After AP (20 patterns):
1. {{p0}}-{{p1}}.{{root}} | Coverage: 50 (kept, high coverage)
2. {{p0}}-{{p1}}-{{p2}}.{{root}} | Coverage: 30 (kept, high coverage)
3. {{p0}}.{{p1}}.{{root}} | Coverage: 40 (kept, high coverage)
... (17 representatives of minor pattern clusters)
```

---

### Step 10: Entropy-Based Selection

**What we do:**
Select the final pattern set using **entropy-based budget allocation** with coverage efficiency analysis.

**The goal:**
Find the smallest set of patterns that achieves target coverage (95%/90%/85% depending on mode).

**How entropy guides selection:**

1. **Structural Diversity (Entropy):**
   Calculate how structurally diverse each pattern's matches are:
   - High entropy = Pattern matches many different structures (good generalization)
   - Low entropy = Pattern matches repetitive structures (may be overfitting)

2. **Coverage Efficiency:**
   Track marginal coverage gain per pattern:
   - First pattern: Might cover 40% of domains (high efficiency)
   - 10th pattern: Might add only 2% coverage (diminishing returns)

3. **Elbow Detection:**
   Find the point where adding more patterns doesn't meaningfully increase coverage:
   ```
   Pattern 1: 40% coverage
   Pattern 2: 60% coverage (+20%)
   Pattern 3: 72% coverage (+12%)
   ...
   Pattern 15: 89% coverage (+0.5%)  ← Elbow detected
   Pattern 16: 89.5% coverage (+0.5%)
   ```

**Why:**
Without intelligent selection:
- Might keep 50+ patterns, most adding <1% coverage
- Patterns with high entropy but low coverage waste computation
- Overfitting patterns (low entropy) generate poor permutations

**Regulator comparison:**
Regulator uses a simple frequency threshold. We use entropy + coverage efficiency because:
- Frequency alone doesn't capture pattern quality
- Subdomain enumeration needs coverage-focused selection
- Elbow detection prevents diminishing returns

**Algorithm:**
```
1. Sort patterns by entropy * coverage (prioritize high-quality patterns)
2. Greedily add patterns while tracking cumulative coverage
3. Stop when:
   - Target coverage reached (95%/90%/85%), OR
   - Elbow detected (marginal gain < sensitivity threshold), OR
   - MaxPatterns reached
```

**Example:**
```
Input: 25 patterns after AP

Entropy-based ranking:
1. {{p0}}-{{p1}}.{{root}} | Entropy: 0.85, Coverage: 50 → Score: 42.5
2. {{p0}}.{{p1}}.{{root}} | Entropy: 0.78, Coverage: 40 → Score: 31.2
3. {{p0}}-{{p1}}-{{p2}}.{{root}} | Entropy: 0.65, Coverage: 25 → Score: 16.25
...

Selection (THOROUGH mode, target 95%):
- Patterns 1-12: Cumulative coverage 94%
- Pattern 13: Would add 0.8% → Elbow detected (< 1% threshold)
- Final: 12 patterns (below 30 max, achieved 94% ≈ 95% target)
```

---

### Step 11: Enrichment

**What we do:**
Make some pattern variables **optional** by adding an empty string (`""`) to their payloads.

**Enrichment rate:** 80% (THOROUGH), 50% (BALANCED/FAST)

**Why:**
Real-world subdomains often have optional components:
- `api.example.com` vs `api-v2.example.com` (version is optional)
- `web.example.com` vs `web-prod.example.com` (environment is optional)

Making variables optional allows one pattern to match both cases.

**How it works:**
```
Original pattern:
{{p0}}-{{p1}}.{{root}}
Payloads: p0=[api,web], p1=[dev,prod]

After enrichment (80% of patterns = this pattern chosen):
{{p0}}-{{p1}}.{{root}}
Payloads: p0=["", api, web], p1=["", dev, prod]

Now generates:
- api-dev.example.com (original)
- api.example.com (p1="")
- web.example.com (p0="", p1="")
- .example.com (both empty, filtered out by validation)
```

**Regulator comparison:**
Regulator doesn't have enrichment. We added this because subdomain patterns often have optional components that pure clustering misses.

**Trade-off:**
Enrichment increases permutation count but improves coverage of edge cases. Higher enrichment rate (THOROUGH) prioritizes coverage; lower rate (FAST) prioritizes speed.

---

## Practical Examples

### Example 1: Small Target (THOROUGH Mode)

**Input:** 50 subdomains from `example.com`
```
api-dev.example.com
api-prod.example.com
api-staging.example.com
web-dev.example.com
web-prod.example.com
cdn-us-east.example.com
cdn-us-west.example.com
cdn-eu-west.example.com
...
```

**Pipeline execution:**
1. Filter: 50 → 48 (removed 2 wildcards)
2. Mode: THOROUGH (< 100 domains)
3. Level grouping: 1 group (all level 2)
4. No sampling (THOROUGH mode)
5. Build indexes: MEMO table (2,304 distances), Trie
6. Three strategies run:
   - Strategy 1 (Global): 4 distance levels × clustering → 12 patterns
   - Strategy 2 (N-gram): Skipped (THOROUGH mode disabled)
   - Strategy 3 (Token): 3 token groups → 8 patterns
7. DSL conversion: 20 raw patterns
8. Deduplication: 20 → 16 patterns
9. AP: Skipped (16 < 30 max)
10. Entropy selection: 16 → 10 patterns (95% coverage achieved)
11. Enrichment: 8 patterns enriched (80% rate)

**Output patterns:**
```
1. {{p0}}-{{p1}}.{{root}} | p0=[api,web,cdn], p1=["",dev,prod,staging]
2. {{p0}}-{{p1}}-{{p2}}.{{root}} | p0=[cdn], p1=[us,eu], p2=["",east,west]
...
```

**Result:** 10 high-quality patterns covering 95% of observed subdomains

---

### Example 2: Large Target (FAST Mode)

**Input:** 5,000 subdomains from `bigcorp.com`

**Pipeline execution:**
1. Filter: 5,000 → 4,850 (removed wildcards, root)
2. Mode: FAST (> 1,000 domains)
3. Level grouping: 4 groups (levels 2-5)
4. Sampling: Level 2 group (2,500 domains) → sampled to 500
5. Build indexes: 4 parallel groups
6. Three strategies per group (parallelized):
   - Strategy 1: Distance levels 2-4 → parallelized
   - Strategy 2: N-gram (enabled, >100 threshold)
   - Strategy 3: Token-level (limited to 30 groups)
7. DSL conversion: 80 raw patterns
8. Deduplication: 80 → 45 patterns
9. AP: 45 → 20 patterns (consolidated)
10. Entropy selection: 20 → 15 patterns (85% coverage)
11. Enrichment: 7 patterns enriched (50% rate)

**Performance:**
- Execution time: ~8 minutes
- Memory peak: ~1.2 GB
- Without optimizations: Would take ~2 hours, ~8 GB memory

**Output:** 15 patterns covering 85% of observed subdomains, focused on high-frequency patterns

---

## Performance Considerations

### Time Complexity (Per Level Group)

| Operation | THOROUGH | BALANCED | FAST | Dominant Factor |
|-----------|----------|----------|------|----------------|
| Edit distance | O(N² × M) | O(N² × M) | O(N² × M) | String length M |
| Strategy 1 | O(N² × K) | O(N² × K) | O(N² × K) parallel | Distance levels K |
| Strategy 2 | Disabled | O(P × N²/P) | O(P × N²/P) parallel | Partitions P |
| Strategy 3 | O(T × N²/T) | O(T × N²/T) | O(T × N²/T) limited | Token groups T |
| AP clustering | O(N² × I) | O(N² × I) | O(N² × I) | Iterations I (6-12) |

**Key insight:** Level-based grouping is critical—without it, N is the full dataset size; with it, N is the largest group size.

### Memory Usage

| Component | Size | Scaling |
|-----------|------|---------|
| MEMO table per group | O(N²) floats | 500 domains = 1 MB |
| Trie per group | O(N × M) | Negligible compared to MEMO |
| Pattern storage | O(P × V) | P patterns, V variables |
| Peak total | | 500MB - 2GB typical |

**Optimization:** Level grouping bounds each MEMO table to O(M²) where M = largest group, preventing O(N²) explosion.

### Parallelization Levels

1. **L1:** Level groups (4-8 groups → 4-8x speedup)
2. **L2:** Strategies (3 strategies → 3x speedup)
3. **L3:** Distance levels in Strategy 1 (FAST mode)
4. **L4:** N-gram partitions in Strategy 2 (FAST mode, >10 partitions)

**Theoretical max speedup:** ~50-100x on 16-core system (real-world: ~20-40x due to Amdahl's law)

### Scalability Limits

| Dataset Size | Mode | Time | Memory | Practical Limit |
|--------------|------|------|--------|-----------------|
| 100 domains | THOROUGH | 30s | 50 MB | ✅ Fast |
| 1,000 domains | BALANCED | 5 min | 500 MB | ✅ Reasonable |
| 10,000 domains | FAST | 30 min | 2 GB | ✅ Manageable |
| 100,000 domains | FAST | 5+ hours | 10+ GB | ⚠️ Requires tuning |

**For very large datasets (>10K domains):**
- Consider pre-filtering to top-level domains only
- Use external sampling before running pattern induction
- Split by root domain and run pattern induction per target

---

## Conclusion

Pattern induction transforms passive enumeration results into actionable intelligence by automatically discovering target-specific naming conventions. The 11-stage pipeline balances accuracy, performance, and scalability through:

- Adaptive modes for different dataset sizes
- Multi-level parallelization for speed
- Intelligent sampling and filtering to reduce noise
- Entropy-based selection for pattern quality
- Enrichment to handle optional components

**Practical takeaway for red teamers:**
1. Run passive enumeration as usual (subfinder, chaos, amass)
2. Feed results to AlterX with `-an` flag
3. Review learned patterns (10-20 patterns typical)
4. Use `-m inferred` or `-m both` to generate focused wordlists
5. Scan with dnsx/massdns for higher hit rates than generic wordlists

The goal isn't to replace manual pattern writing entirely—it's to automate 80% of the work and surface patterns you might have missed.
