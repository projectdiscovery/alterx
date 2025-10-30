# Regulator Pattern Induction Algorithm

**File:** `regulator.py`
**Purpose:** Infer DNS subdomain patterns from passive enumeration results using edit distance clustering

---

## Core Data Structures

### 1. Token Structure
```
Subdomain: api-dev-01.staging.example.com

After tokenization:
[
  ["api", "-dev", "-01"],    // Level 0 (leftmost subdomain)
  ["staging"]                 // Level 1 (second subdomain level)
]

Key structure elements:
- Levels: separated by dots, indexed 0 (leftmost) to N (rightmost before TLD)
- Positions: within each level, indexed 0 to M
- Tokens: individual parts (words, hyphenated words, numbers)
```

### 2. Edit Distance Closure
```
A set of subdomains grouped by similarity threshold (delta)

Example with delta=3:
{
  "api-dev-01.example.com",
  "api-dev-02.example.com",
  "api-dev-03.example.com"
}
// All within 3 character edits of each other
```

### 3. Level-Position Map
```
Structure used by closure_to_regex():

levels = {
  0: {  // Level 0
    0: {"api"},              // Position 0: always "api"
    1: {"-dev"},             // Position 1: always "-dev"
    2: {"-01", "-02", "-03"} // Position 2: VARIES (becomes alternation)
  },
  1: {  // Level 1
    0: {"staging", None}     // Position 0: optional (None means missing)
  }
}

optional = {
  0: {
    0: ["api", "api", "api"],
    1: ["-dev", "-dev", "-dev"],
    2: ["-01", "-02", "-03"]
  },
  1: {
    0: ["staging", "", ""]   // Empty strings indicate level is optional
  }
}
```

---

## Algorithm Phases

### Phase 1: Tokenization (`tokenize()`)

**Goal:** Break subdomains into structured tokens preserving hierarchy and separators.

**Input:** List of full domain names
**Output:** 2D array of tokens per domain

**Process:**

1. **Extract subdomain part** (remove TLD using tldextract)
   ```
   api-dev-01.staging.example.com
   → subdomain: "api-dev-01.staging"
   ```

2. **Split by dots** (DNS levels)
   ```
   "api-dev-01.staging"
   → ["api-dev-01", "staging"]
   ```

3. **For each level, split by dashes** (preserve dash position)
   ```
   "api-dev-01"
   → split("-") = ["api", "dev", "01"]
   → prefix all but first with "-": ["api", "-dev", "-01"]
   ```

4. **Split each token by numbers** (preserve number sequences)
   ```
   "api01"
   → re.split('([0-9]+)', "api01") = ["api", "01"]

   "server123"
   → ["server", "123"]
   ```

5. **Special case: hyphenated numbers**
   ```
   "foo-12.example.com"
   → tokens: ["foo", "-12"]
   // NOT ["foo", "-", "12"]
   ```

**Examples:**

```
Input: api-dev-01.staging.example.com
Output: [["api", "-dev", "-01"], ["staging"]]

Input: web-us-east-1.example.com
Output: [["web", "-us", "-east", "-1"]]

Input: db01.prod.internal.example.com
Output: [["db", "01"], ["prod"], ["internal"]]

Input: cdn.example.com
Output: [["cdn"]]

Input: api123test.example.com
Output: [["api", "123", "test"]]
```

**Why this structure matters:**
- Preserves where dashes appear (semantic meaning)
- Keeps numbers separate (for range detection)
- Maintains DNS hierarchy (level 0 vs level 1)
- Allows positional comparison across similar subdomains

---

### Phase 2: Edit Distance Closures (`edit_closures()`)

**Goal:** Group subdomains that are structurally similar using edit distance metric.

**Input:** List of subdomains, delta (max edit distance)
**Output:** List of sets (closures), where each set contains similar subdomains

**Edit Distance:** Minimum number of single-character edits (insert, delete, substitute) to transform string A into string B.

**Process:**

1. **Precompute all pairwise distances** (optimization - done once in main())
   ```python
   for s, t in combinations_with_replacement(known_hosts, 2):
       MEMO[s+t] = editdistance.eval(s, t)
   ```

   This builds a lookup table so we never recompute distances.

2. **For each subdomain, find its neighbors**
   ```python
   for a in items:
       neighbors = set([a])
       for b in items:
           if distance(a, b) < delta:
               neighbors.add(b)
   ```

3. **Deduplicate identical closures**
   ```python
   for existing_closure in results:
       if new_closure == existing_closure:
           skip it  // Already found this group
   ```

**Example with delta=3:**

```
Subdomains:
  api-dev-01.example.com
  api-dev-02.example.com
  api-dev-03.example.com
  api-prod-01.example.com
  web-staging.example.com

Distances:
  api-dev-01 ↔ api-dev-02: 1 char diff
  api-dev-01 ↔ api-dev-03: 1 char diff
  api-dev-01 ↔ api-prod-01: 2 char diff
  api-dev-01 ↔ web-staging: 12 char diff

Closure 1 (around api-dev-01):
  {api-dev-01, api-dev-02, api-dev-03, api-prod-01}
  // All within 3 edits

Closure 2 (around web-staging):
  {web-staging}
  // No neighbors within 3 edits
```

**Example with delta=7:**

```
Same subdomains, larger delta:

Closure 1 (around api-dev-01):
  {api-dev-01, api-dev-02, api-dev-03, api-prod-01}
  // Still grouped

Closure 2 (around web-staging):
  {web-staging}
  // Still alone (12 edits away from api group)
```

**Why multiple delta values?**

The algorithm tries k=2, k=3, ..., k=10 to find patterns at different similarity levels:
- **Low delta (k=2-3):** Catches very similar variations (api01 vs api02)
- **High delta (k=7-10):** Catches broader structural patterns (api-dev vs api-prod)

---

### Phase 3: Pattern Generation (`closure_to_regex()`)

**Goal:** Convert a closure (set of similar subdomains) into a generalized regex pattern.

**Input:** Domain (TLD), closure (list of similar subdomains)
**Output:** Regex pattern string

**Process:**

#### Step 1: Build Level-Position Map

For each subdomain in closure:
1. Tokenize it
2. For each level (0, 1, 2, ...), for each position (0, 1, 2, ...)
3. Record what token appears at that level-position
4. Track if that position exists in all subdomains (for optionality)

**Example:**

```
Closure:
  api-dev-01.example.com
  api-dev-02.example.com
  api-prod-03.example.com

Tokenized:
  [["api", "-dev", "-01"]]
  [["api", "-dev", "-02"]]
  [["api", "-prod", "-03"]]

Level-Position Map:
  Level 0:
    Position 0: {"api"} → appears in all 3
    Position 1: {"-dev", "-prod"} → VARIES
    Position 2: {"-01", "-02", "-03"} → VARIES

Optional tracking:
  Level 0, Position 0: [api, api, api] → count=3, NOT optional
  Level 0, Position 1: [-dev, -dev, -prod] → count=3, NOT optional
  Level 0, Position 2: [-01, -02, -03] → count=3, NOT optional
```

#### Step 2: Generate Regex Parts

For each level:
```python
for level in levels:
    for position in level:
        tokens_at_position = levels[level][position]

        if len(tokens_at_position) == 1:
            # Single value - no alternation needed
            add_to_regex(the_token)
        else:
            # Multiple values - create alternation
            add_to_regex(f"({token1}|{token2}|{token3})")

        # Check if position is optional
        if some_subdomains_missing_this_position:
            add_to_regex("?")
```

**Example continuing from above:**

```
Level 0:
  Position 0: api (single value)
  Position 1: (-dev|-prod) (alternation)
  Position 2: (-01|-02|-03) (alternation)

Regex so far: api(-dev|-prod)(-01|-02|-03)
```

#### Step 3: Handle Optional Levels

A level is optional if:
- Not all subdomains have that level, OR
- The level has distinct values across subdomains

**Example with optional level:**

```
Closure:
  api.staging.example.com
  api.example.com
  api.prod.example.com

Tokenized:
  [["api"], ["staging"]]
  [["api"], []]
  [["api"], ["prod"]]

Level 0:
  Position 0: {"api"} → always present

Level 1:
  Position 0: {"staging", None, "prod"}
  // None indicates missing level

Is Level 1 optional?
  - Not all subdomains have it (second one missing)
  - Values are distinct ({staging, prod})
  YES → Make it optional

Regex: api(.staging|.prod)?.example.com
```

#### Step 4: Combine Levels with Dots

```python
if level > 0:
    prefix_with_dot = True

# First level: no dot
regex = "api(-dev|-prod)"

# Second level: add dot
regex = "api(-dev|-prod).staging"

# Optional level: wrap in parens with ?
regex = "api(-dev|-prod)(.staging)?"
```

**Full examples:**

```
Example 1:
Closure: {api-dev-01, api-dev-02, api-dev-03}
Regex: api-dev-(01|02|03).example.com

Example 2:
Closure: {api.staging, api.prod, web.staging}
Regex: (api|web).(staging|prod).example.com

Example 3:
Closure: {db01.prod, db02.prod, db03.prod, db04}
Level 1 analysis:
  - db01, db02, db03 have level 1 ("prod")
  - db04 doesn't have level 1
  → Level 1 is optional
Regex: db(01|02|03|04)(.prod)?.example.com

Example 4:
Closure: {api.v1.staging, api.v2.staging, api.staging}
Level 1 has distinct values: {v1, v2, None}
Level 2 always "staging"
Regex: api(.v1|.v2)?.staging.example.com
```

---

### Phase 4: Number Range Compression (`compress_number_ranges()`)

**Goal:** Optimize regex by replacing long number lists with compact ranges.

**Problem:**
```
(01|02|03|04|05|06|07|08|09|10|11|12) → Too verbose
```

**Solution:**
```
([0-1][0-9]) → Generates same numbers, much shorter
```

**Process:**

#### Step 1: Find Number Groups

Scan regex for groups containing only numbers:
```python
for each (group) in regex:
    if group contains only numeric alternations:
        candidates.add(group)
```

**Example:**
```
Input: api-(01|02|03|04|05).staging-(dev|prod).example.com

Groups found:
  Group 1: (01|02|03|04|05) ✓ All numeric
  Group 2: (dev|prod) ✗ Not numeric

Process only Group 1
```

#### Step 2: Analyze Digit Positions

Reverse numbers (DNS interprets right-to-left) and analyze each digit position:

```
Numbers: 01, 02, 03, 04, 05

Reversed: 10, 20, 30, 40, 50

Position analysis:
  Position 0 (ones): {0} → single value
  Position 1 (tens): {1, 2, 3, 4, 5} → range [1-5]

Result: [1-5]0
```

**More complex example:**

```
Numbers: 1, 2, 10, 11, 12, 20, 21

Reversed: 1, 2, 01, 11, 21, 02, 12

Position 0 (ones):
  From "1": position 0 = 1
  From "2": position 0 = 2
  From "01": position 0 = 1
  From "11": position 0 = 1
  From "21": position 0 = 1
  From "02": position 0 = 2
  From "12": position 0 = 2
  → {1, 2} → [1-2]

Position 1 (tens):
  From "1": no position 1 → None
  From "2": no position 1 → None
  From "01": position 1 = 0
  From "11": position 1 = 1
  From "21": position 1 = 2
  From "02": position 1 = 0
  From "12": position 1 = 1
  → {None, 0, 1, 2}
  → Has None, so this position is OPTIONAL
  → [0-2]?

Final: [0-2]?[1-2]
```

#### Step 3: Handle Hyphenated Numbers

Special case: numbers with leading dash (e.g., -01, -02, -03)

```
Input: api(-01|-02|-03).example.com

Detection: All start with "-"
Process:
  - Strip the dash
  - Compress: 01, 02, 03 → [0-1][0-3]
  - Re-add dash: (-[0-1][0-3])

Output: api(-[0-1][0-3]).example.com
```

#### Step 4: Mixed Groups (numbers + non-numbers)

If a group has both numbers and non-numeric tokens:

```
Input: (01|02|03|dev|prod)

Numbers: {01, 02, 03}
Non-numbers: {dev, prod}

Compress numbers: ([0-1][0-3])
Keep non-numbers: (dev|prod)

Output: (([0-1][0-3])|(dev|prod))
```

**Full Examples:**

```
Before: api-(1|2|3|4|5|6|7|8|9).example.com
After:  api-([1-9]).example.com

Before: server-(01|02|03|04|05|06|07|08|09|10).example.com
After:  server-([0-1][0-9]).example.com

Before: db-(001|010|100).example.com
After:  db-(001|010|100).example.com  // Can't compress - not sequential

Before: web-(1|2|3|10|11|12|20|21).example.com
After:  web-([0-2]?[1-2]).example.com

Before: api(-01|-02|-03|-04|-05).staging.example.com
After:  api(-[0-1][0-5]).staging.example.com
```

---

### Phase 5: Quality Filtering (`is_good_rule()`)

**Goal:** Reject patterns that would generate an unreasonable number of subdomains compared to what was actually observed.

**The Problem:**

Some patterns might be technically correct but generate millions of possibilities:

```
Observed: {a01, b02, c03}
Bad pattern: [a-z][0-9]{2}.example.com
Generates: 26 × 100 = 2,600 possibilities
Only observed: 3
Ratio: 866:1 → WAY TOO BROAD
```

**The Solution: Ratio Test**

```
ratio = (possible_generations / observed_count)

if ratio < max_ratio:
    ACCEPT pattern
else:
    REJECT pattern
```

**Process:**

#### Step 1: Count Possible Generations

Uses `DankEncoder` to calculate how many subdomains a regex can generate:

```python
e = DankEncoder(regex, 256)
nwords = e.num_words(1, 256)
```

This counts all possible strings the regex can generate within length bounds.

**Example:**

```
Pattern: api-(dev|prod).example.com
Possible generations: 2 (api-dev, api-prod)

Pattern: api-[0-9]{2}.example.com
Possible generations: 100 (api-00 through api-99)

Pattern: [a-z]{5}.example.com
Possible generations: 26^5 = 11,881,376
```

#### Step 2: Apply Threshold

**Two-tier filtering:**

1. **Absolute threshold** (default: 500)
   ```
   if nwords < 500:
       AUTO-ACCEPT  // Small pattern, definitely safe
   ```

2. **Ratio test** (default max_ratio: 25.0)
   ```
   if nwords >= 500:
       if (nwords / observed_count) < 25.0:
           ACCEPT
       else:
           REJECT  // Too broad
   ```

**Examples:**

```
Example 1: Small Pattern
Pattern: api-(dev|prod|staging).example.com
Observed: {api-dev, api-prod, api-staging}
Generations: 3
Threshold check: 3 < 500 ✓
Result: AUTO-ACCEPT

Example 2: Medium Pattern - Good Ratio
Pattern: api-[0-9]{2}.example.com
Observed: {api-01, api-02, api-03, api-04, api-05}
Generations: 100
Threshold check: 100 < 500 ✓
Result: AUTO-ACCEPT

Example 3: Large Pattern - Good Ratio
Pattern: api-[0-5][0-9].example.com
Observed: 30 subdomains (api-01 through api-30)
Generations: 60
Threshold check: 60 < 500 ✓
Result: AUTO-ACCEPT

Example 4: Large Pattern - Acceptable Ratio
Pattern: (api|web|cdn)-[a-z]{3}.example.com
Observed: 1000 subdomains
Generations: 3 × 17,576 = 52,728
Ratio: 52,728 / 1000 = 52.7
Threshold check: 52,728 >= 500, check ratio
Ratio check: 52.7 > 25.0 ✗
Result: REJECT (too broad)

Example 5: Large Pattern - Good Ratio
Pattern: api-(us|eu)-(east|west)-[1-3].example.com
Observed: 10 subdomains
Generations: 2 × 2 × 3 = 12
Ratio: 12 / 10 = 1.2
Threshold check: 12 < 500 ✓
Result: AUTO-ACCEPT
```

**Why This Matters:**

Without quality filtering:
```
Observed: {test1, test2}
Generated pattern: test[0-9]+.example.com
This would generate: test1, test2, test3, ..., test999999, ...
Millions of invalid subdomains!
```

With quality filtering:
```
Observed: {test1, test2}
Generated pattern: test[0-9]+.example.com
Generations: ∞ (unbounded)
Ratio: ∞ > 25.0 ✗
REJECTED

Alternative generated: test[1-2].example.com
Generations: 2
Ratio: 1.0 ✓
ACCEPTED
```

**Tuning Parameters:**

- **Lower max_ratio (e.g., 10):** Stricter, fewer false positives, may miss valid patterns
- **Higher max_ratio (e.g., 50):** Looser, more patterns, more false positives
- **Lower threshold (e.g., 100):** More patterns auto-accepted
- **Higher threshold (e.g., 1000):** More patterns go through ratio test

---

## Multi-Level Domain Handling

### The Challenge

**Problem:** Subdomains can have different numbers of levels.

```
api.example.com          → 1 level
api.staging.example.com  → 2 levels
api.v1.staging.example.com → 3 levels
```

**Question:** Can these be grouped together? How are patterns generated?

### Level Matching Strategy

#### Rule 1: Levels are Aligned Left-to-Right

The algorithm treats level 0 (leftmost) as the primary level:

```
Level 0 (leftmost):
  api.example.com          → "api"
  api.staging.example.com  → "api"
  web.prod.example.com     → "web"

These CAN be grouped if level 0 matches or is similar.
```

#### Rule 2: Missing Levels are Treated as Optional

When building the level-position map:

```
Closure:
  api.staging.example.com
  api.prod.example.com
  api.example.com

Tokenized:
  [["api"], ["staging"]]
  [["api"], ["prod"]]
  [["api"], []]             ← No level 1

Level analysis:
  Level 0: {"api"} in all 3 → required
  Level 1: {"staging", "prod", None} → OPTIONAL (one subdomain missing it)

Generated: api(.staging|.prod)?.example.com
```

### Examples with Level Mismatches

#### Example 1: 1-level vs 2-level

```
Input:
  api.example.com
  api.staging.example.com
  api.prod.example.com

Edit distances:
  api ↔ api.staging: ~8 chars (too far with low delta)

With delta=10:
  Closure: {api, api.staging, api.prod}

Pattern generation:
  Level 0: {"api"}
  Level 1: {None, "staging", "prod"}

Result: api(.staging|.prod)?.example.com

Generates:
  api.example.com ✓
  api.staging.example.com ✓
  api.prod.example.com ✓
```

#### Example 2: 2-level vs 3-level

```
Input:
  api.staging.example.com
  api.v1.staging.example.com
  api.v2.staging.example.com

Edit distances (with delta=10):
  Closure: {api.staging, api.v1.staging, api.v2.staging}

Tokenized:
  [["api"], ["staging"]]
  [["api"], ["v1"], ["staging"]]
  [["api"], ["v2"], ["staging"]]

Level analysis:
  Level 0: {"api"} → all have it
  Level 1: {"staging", "v1", "v2"} → all have it but DIFFERENT values
  Level 2: {None, "staging", "staging"} → OPTIONAL (first subdomain missing it)

Pattern generation:
  Level 0: api
  Level 1: (staging|v1|v2)
  Level 2: (.staging)?

Result: api.(staging|v1|v2)(.staging)?.example.com

BUT WAIT - this is wrong! Let's trace through the actual algorithm:
```

#### Actual Algorithm Behavior (Corrected)

The algorithm processes levels sequentially:

```
Input:
  api.v1.staging.example.com
  api.v2.staging.example.com
  api.staging.example.com

Tokenized:
  [["api"], ["v1"], ["staging"]]  → 3 levels
  [["api"], ["v2"], ["staging"]]  → 3 levels
  [["api"], ["staging"]]          → 2 levels

Level-Position mapping:

Level 0:
  All 3 have position 0 = "api"

Level 1:
  First has position 0 = "v1"
  Second has position 0 = "v2"
  Third has position 0 = "staging"
  → {"v1", "v2", "staging"}

Level 2:
  First has position 0 = "staging"
  Second has position 0 = "staging"
  Third has no level 2 → None
  → {"staging", None}

Is Level 1 optional?
  All 3 subdomains have level 1 → NOT missing
  But values are distinct → YES, optional

Is Level 2 optional?
  Only 2 of 3 have level 2 → YES, optional

Generated: api.(v1|v2|staging)(.staging)?.example.com

This generates:
  api.v1.example.com
  api.v1.staging.example.com ✓
  api.v2.example.com
  api.v2.staging.example.com ✓
  api.staging.example.com ✓
  api.staging.staging.example.com ✗ (invalid but generated)
```

**The Issue:** This creates invalid combinations!

**Solution in Code:** The algorithm likely generates separate patterns for different level structures:

```
Strategy 1: Group by level count first

Group A (3 levels):
  api.v1.staging.example.com
  api.v2.staging.example.com
  → api.(v1|v2).staging.example.com

Group B (2 levels):
  api.staging.example.com
  → api.staging.example.com (single item, no pattern)

OR

Strategy 2: Use higher delta to group similar structures:

With delta=5:
  Closure 1: {api.v1.staging, api.v2.staging}
  → api.(v1|v2).staging.example.com

  Closure 2: {api.staging}
  → api.staging.example.com
```

#### Example 3: Completely Different Levels

```
Input:
  api.dev.internal.example.com (3 levels)
  web.prod.example.com (2 levels)

Edit distances:
  api.dev.internal ↔ web.prod: ~17 chars

Even with delta=10: Too far apart
  → Separate closures
  → Separate patterns

Closure 1: {api.dev.internal}
Pattern: api.dev.internal.example.com

Closure 2: {web.prod}
Pattern: web.prod.example.com
```

### Level Handling Summary

**Key Points:**

1. **Closures respect edit distance across entire subdomain string**
   - `api.example.com` vs `api.staging.example.com` has large edit distance
   - Unlikely to be in same closure with low delta

2. **Level optionality is detected automatically**
   - If any subdomain in closure missing a level → that level is optional
   - Marked with `?` in regex

3. **Different level structures usually result in separate patterns**
   - Unless delta is high enough to group them
   - Quality filtering may reject overly broad patterns

4. **Best results when subdomains have similar structure**
   - Same number of levels
   - Similar content at each level
   - Edit distance detects this naturally

---

## Multi-Strategy Search (Main Loop)

The algorithm combines three complementary strategies to find patterns at different granularities.

### Strategy 1: Global Edit Distance Clustering

**What:** Cluster ALL subdomains by edit distance, iterating through different delta values.

**When it works well:**
- Finding variations in simple patterns
- Detecting number sequences
- Identifying environment suffixes (dev, prod, staging)

**Code:**
```python
for k in range(dist_low, dist_high):  # k=2 to k=10
    closures = edit_closures(known_hosts, delta=k)
    for closure in closures:
        if len(closure) > 1:
            pattern = closure_to_regex(domain, closure)
            if is_good_rule(pattern):
                accept_pattern(pattern)
```

**Example:**

```
Input (100 subdomains):
  api-dev-01, api-dev-02, ..., api-dev-99
  web-prod-01, web-prod-02, ..., web-prod-50
  db-staging-01, db-staging-02, ..., db-staging-20

With k=3 (tight clustering):
  Closure A: {api-dev-01, api-dev-02, api-dev-03}
  Closure B: {api-dev-04, api-dev-05, api-dev-06}
  ...
  → Many small patterns

With k=7 (looser clustering):
  Closure A: {api-dev-01 through api-dev-99}
  Closure B: {web-prod-01 through web-prod-50}
  Closure C: {db-staging-01 through db-staging-20}
  → Fewer, broader patterns

Generated patterns:
  api-dev-[0-9]{2}.example.com
  web-prod-[0-5][0-9].example.com
  db-staging-([0-1][0-9]|20).example.com
```

### Strategy 2: N-gram Prefix Anchoring

**What:** Use a trie data structure to find all subdomains starting with specific 1-2 character prefixes, then generate patterns for each prefix group.

**Why n-grams:** Efficiently groups subdomains by their starting characters without full edit distance calculation.

**Code:**
```python
# Build all 1-grams and 2-grams
ngrams = ['a', 'b', ..., 'z', '0', ..., '9', 'aa', 'ab', ..., 'zz', '00', ...]

for ngram in ngrams:
    keys = trie.keys(ngram)  # All subdomains starting with ngram
    if len(keys) > 0:
        pattern = closure_to_regex(domain, keys)
        if is_good_rule(pattern):
            accept_pattern(pattern)
```

**When it works well:**
- Common prefixes (api-, web-, cdn-, db-)
- Short prefix variations
- Fast initial pattern discovery

**Example:**

```
Trie contains:
  api-dev-01.example.com
  api-dev-02.example.com
  api-prod-01.example.com
  apigateway.example.com
  web-staging.example.com
  web-prod.example.com

N-gram: "a"
  Keys: None (too broad, likely skipped)

N-gram: "ap"
  Keys: {api-dev-01, api-dev-02, api-prod-01, apigateway}
  Pattern: ap(i(-dev|-prod)-[0-1][0-2]|igateway).example.com
  Quality check: Generates ~10 subdomains, observed 4
  Result: ACCEPT

N-gram: "we"
  Keys: {web-staging, web-prod}
  Pattern: web-(staging|prod).example.com
  Quality check: Generates 2, observed 2
  Result: ACCEPT
```

**Trie Structure Benefits:**

```
Trie visualization:
  a → p → i → -dev-01
          i → -dev-02
          i → -prod-01
          i → gateway
  w → e → b → -staging
          b → -prod

Fast prefix lookup: trie.keys("ap") → O(1) lookup, returns all branches
```

### Strategy 3: Token-Level Prefix + Edit Distance

**What:** Combine prefix matching with edit distance clustering for fine-grained patterns.

**Process:**
1. Extract first tokens from all subdomains
2. For each unique token prefix, get all matching subdomains
3. Apply edit distance clustering within each prefix group
4. Check for redundant prefixes (e.g., "api" vs "api-dev")

**Code:**
```python
for ngram in ngrams:
    keys = trie.keys(ngram)

    # Extract first tokens
    prefixes = [first_token(key) for key in keys]

    for prefix in prefixes:
        keys = trie.keys(prefix)

        # Apply edit distance within prefix group
        for k in range(dist_low, dist_high):
            closures = edit_closures(keys, delta=k)
            for closure in closures:
                pattern = closure_to_regex(domain, closure)
                if is_good_rule(pattern):
                    accept_pattern(pattern)
```

**When it works well:**
- Complex multi-part prefixes (api-dev-, web-us-, cdn-prod-)
- Regional variants (us-east, eu-west)
- Nested variations within prefix groups

**Example:**

```
Input:
  api-dev-01.example.com
  api-dev-02.example.com
  api-dev-us.example.com
  api-dev-eu.example.com
  api-prod-01.example.com
  api-prod-02.example.com

Step 1: N-gram "api" gets all 6 subdomains
  Try global pattern: api-(dev|prod)-(01|02|us|eu).example.com
  Quality check: Generates 8, observed 6
  Ratio: 1.33 ✓ ACCEPT (but not optimal)

Step 2: Extract token prefixes
  first_token("api-dev-01") = "api-dev"
  first_token("api-prod-01") = "api-prod"
  Prefixes: {"api-dev", "api-prod"}

Step 3: Process "api-dev" prefix
  Keys: {api-dev-01, api-dev-02, api-dev-us, api-dev-eu}

  With k=3:
    Closure A: {api-dev-01, api-dev-02}
    Closure B: {api-dev-us, api-dev-eu}

  Pattern A: api-dev-(01|02).example.com
  Pattern B: api-dev-(us|eu).example.com

Step 4: Process "api-prod" prefix
  Keys: {api-prod-01, api-prod-02}
  Pattern: api-prod-(01|02).example.com

Result: 3 more specific patterns instead of 1 broad pattern
```

**Redundancy Check:**

```
Problem:
  Prefix "api" → generates pattern
  Prefix "api-dev" → generates pattern (more specific)

Without check:
  Pattern 1: api-(dev|prod)-.*.example.com
  Pattern 2: api-dev-(01|02).example.com
  → Pattern 2 is redundant (subset of Pattern 1)

With check (line 271-275):
  if prefix.startswith(last_accepted_prefix):
      REJECT as redundant
```

---

## Complete Algorithm Flow

```
INPUT: List of observed subdomains, target domain

STEP 1: Tokenization
  ↓ For each subdomain
  ↓ Extract subdomain part (remove TLD)
  ↓ Split by dots, dashes, numbers
  ↓ Build structured token array
  ↓
  [["api", "-dev", "-01"], ["staging"]]

STEP 2: Build Edit Distance Table (memoization)
  ↓ For all pairs of subdomains
  ↓ Compute edit distance
  ↓ Store in MEMO[s+t]
  ↓
  {"api-dev-01api-dev-02": 1, "api-dev-01web-prod": 12, ...}

STEP 3: Build Trie Index
  ↓ For each subdomain
  ↓ Insert into trie
  ↓
  Trie[api-dev-01] = True

STEP 4: Strategy 1 - Global Clustering
  ↓ For k=2,3,...,10
  ↓   Find edit closures (delta=k)
  ↓   For each closure (if size > 1)
  ↓     Generate regex pattern
  ↓     Check quality (ratio test)
  ↓     If good: add to patterns
  ↓
  Patterns: ["api-dev-[0-9]{2}.example.com", ...]

STEP 5: Strategy 2 - N-gram Prefix
  ↓ For each 1-gram, 2-gram (a, b, ..., aa, ab, ...)
  ↓   Get all subdomains starting with n-gram
  ↓   Generate regex pattern
  ↓   Check quality
  ↓   If good: add to patterns
  ↓
  Patterns: [..., "api-(dev|prod).example.com", ...]

STEP 6: Strategy 3 - Token Prefix + Clustering
  ↓ For each n-gram
  ↓   Get subdomains starting with n-gram
  ↓   Extract first token prefixes
  ↓   For each prefix
  ↓     Get subdomains with that prefix
  ↓     For k=2,3,...,10
  ↓       Find edit closures within prefix group
  ↓       Generate patterns
  ↓       Check quality
  ↓       Check redundancy
  ↓       If good: add to patterns
  ↓
  Patterns: [..., "api-dev-(us|eu).example.com", ...]

STEP 7: Deduplicate Patterns
  ↓ new_rules = set(all_patterns)
  ↓
  Final unique patterns

STEP 8: Generate Subdomains (optional)
  ↓ For each pattern
  ↓   Use DankGenerator to expand regex
  ↓   Output all possible subdomains
  ↓
  Output wordlist

OUTPUT: Set of regex patterns + generated subdomain wordlist
```

---

## Key Algorithm Insights

### 1. Why Edit Distance Works Better Than Exact Matching

**Traditional approaches:**
```
Look for exact substring matches:
  "api-dev" appears in api-dev-01, api-dev-02
  → Pattern: api-dev-*

Misses:
  api-dev vs api-prod (only 1 char different!)
```

**Edit distance approach:**
```
api-dev-01 ↔ api-prod-01: distance = 2
  → Grouped together with delta ≥ 2
  → Pattern: api-(dev|prod)-01
```

### 2. Why Multiple Strategies Are Needed

Each strategy catches different pattern types:

**Strategy 1 (Global):**
- Best for: Simple variations across entire dataset
- Example: All subdomains with numbers (01-99)

**Strategy 2 (N-gram):**
- Best for: Common prefixes
- Example: All "api-" subdomains

**Strategy 3 (Token + Distance):**
- Best for: Complex nested structures
- Example: api-dev-us vs api-dev-eu vs api-prod-us

**Real-world example:**
```
Input: 1000 subdomains with patterns like:
  api-dev-us-east-1
  api-dev-us-east-2
  api-dev-eu-west-1
  web-prod-01
  db-staging-primary

Strategy 1 finds: High-level groups (api vs web vs db)
Strategy 2 finds: Prefix groups (api-dev-, api-prod-, web-prod-)
Strategy 3 finds: Nested structures (us-east-1, us-east-2, eu-west-1)

Result: Comprehensive pattern coverage
```

### 3. Why Quality Filtering Is Critical

**Without filtering:**
```
Observed: {test1.example.com, test2.example.com}
Generated: [a-z]+[0-9]+.example.com
Expands to: MILLIONS of subdomains
```

**With filtering:**
```
Observed: {test1, test2}
Bad pattern: [a-z]+[0-9]+
  Ratio: ∞ / 2 → REJECTED

Better pattern: test[1-2]
  Ratio: 2 / 2 = 1.0 → ACCEPTED
```

### 4. Why Memoization Matters

**Edit distance calculation is expensive:** O(m×n) for strings of length m and n

**Without memoization:**
```
1000 subdomains → 1,000,000 pairwise comparisons
Each comparison: ~1ms
Total: ~1000 seconds (16+ minutes)
```

**With memoization:**
```
Precompute once: ~1000 seconds
All lookups: O(1)
Total per lookup: ~0.000001ms
Multiple strategies can reuse same table
```

---

## Summary

**Core Innovation:** Edit distance clustering discovers structural similarities without hardcoded patterns.

**Key Steps:**
1. Tokenize → preserve structure
2. Cluster by similarity → find groups
3. Align variations → identify what changes
4. Generate patterns → create regex
5. Optimize → compress numbers
6. Filter → reject too-broad patterns

**Multi-strategy approach:** Combines global clustering, prefix matching, and token-level analysis for comprehensive coverage.

**Quality control:** Ratio test prevents pattern explosion while allowing valid variations.

**Result:** High-quality, target-specific patterns that reflect actual infrastructure naming conventions.
