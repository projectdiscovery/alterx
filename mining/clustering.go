package mining

import (
	"sort"
	"strings"
)

// hierarchicalNgramClustering clusters subdomains using a hierarchical approach that combines
// ngram prefix matching, token extraction, and edit distance clustering.
//
// HIERARCHICAL ALGORITHM:
// For each ngram ('a', 'ab', 'b0', etc.):
//  1. Get keys (hostnames) starting with ngram: keys = trie.keys(ngram)
//  2. Chance 1: Generate pattern from ALL ngram keys directly
//  3. Extract prefixes: Get first token from each hostname
//     - first_token("api-prod-1.example.com") → "api"
//  4. For each unique prefix:
//     a. Get new keys: keys = trie.keys(prefix) (all hostnames starting with this prefix)
//     b. Chance 2: Generate pattern from ALL prefix keys directly
//     c. Chance 3 (if prefix length > 1): Do edit distance clustering
//     - For each k value, compute closures on prefix keys
//     - For each closure, generate pattern
//
// HIERARCHY: ngram → keys → prefixes → new keys per prefix → edit distance clustering → patterns
//
// EXAMPLE:
// Given ngram "a" with keys: ["api-prod-1", "api-prod-2", "app-dev-1"]
// Chance 1: Generate pattern from all 3 keys
// Extract prefixes: ["api", "api", "app"] → unique: ["api", "app"]
// For prefix "api":
//   - Get keys starting with "api": ["api-prod-1", "api-prod-2"]
//   - Chance 2: Generate pattern from these 2 keys
//   - Chance 3: Since len("api") > 1, do edit distance clustering with k=2,3,...
//
// For prefix "app":
//   - Get keys starting with "app": ["app-dev-1"]
//   - Chance 2: Generate pattern (single item, might be skipped)
//   - Chance 3: Since len("app") > 1, do edit distance clustering
func (p *PatternMiner) hierarchicalNgramClustering() error {
	// Generate all possible unigrams and bigrams for valid subdomain prefixes
	unigrams, bigrams := GenerateValidNgrams()

	// Combine all ngrams for processing
	allNgrams := append([]string{}, unigrams...)
	allNgrams = append(allNgrams, bigrams...)

	// Process each ngram hierarchically
	for _, ngram := range allNgrams {
		if err := p.processNgramHierarchy(ngram); err != nil {
			return err
		}
	}

	return nil
}

// processNgramHierarchy processes a single ngram through the hierarchical clustering pipeline.
//
// ALGORITHM (matches Python reference):
// 1. Get all keys matching the ngram prefix
// 2. Generate pattern from ngram-level keys (Chance 1)
// 3. Extract first tokens to get unique prefixes (sorted)
// 4. Process each prefix level with redundancy filtering
//
// REDUNDANCY FILTERING (matches Python):
//   Python: if last is None or not prefix.startswith(last)
//   Skip prefix if it starts with the previous prefix (redundant)
func (p *PatternMiner) processNgramHierarchy(ngram string) error {
	// Step 1: Get all keys (subdomains) starting with this ngram
	ngramKeys := p.getSubdomainsByPrefix(ngram)
	if len(ngramKeys) == 0 {
		return nil
	}

	// Step 2: Chance 1 - Generate and store pattern from ALL ngram keys directly
	p.tryGenerateAndStorePattern(ngramKeys)

	// Step 3: Extract first tokens (prefixes) from all keys
	prefixMap := make(map[string]struct{})
	for _, key := range ngramKeys {
		prefix := p.extractFirstToken(key)
		if prefix != "" {
			prefixMap[prefix] = struct{}{}
		}
	}

	// Convert to sorted slice for redundancy filtering
	// Python: prefixes = sorted(list(set([first_token(k) for k in trie.keys(ngram)])))
	prefixes := make([]string, 0, len(prefixMap))
	for prefix := range prefixMap {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	// Step 4: Process each prefix with redundancy filtering
	// Python: last = None
	//         if last is None or not prefix.startswith(last):
	//           if pattern_added:  # Only update last when pattern is actually added!
	//             last = prefix
	//         else:
	//           logging.warning(f"Rejecting redundant prefix: {prefix}")
	//           continue
	var last string
	for _, prefix := range prefixes {
		// Skip if this prefix starts with the previous one (redundant)
		if last != "" && strings.HasPrefix(prefix, last) {
			// Redundant prefix, skip it
			continue
		}

		// Process this prefix and check if any pattern was added
		patternAdded := p.processPrefixLevel(prefix)

		// Only update last if a pattern was actually added (matches Python behavior)
		if patternAdded {
			last = prefix
		}
	}

	return nil
}

// processPrefixLevel processes clustering at the prefix level.
//
// ALGORITHM (matches Python reference):
// 1. Get all keys matching the prefix
// 2. Generate pattern from prefix-level keys (Chance 2)
// 3. If prefix length > 1, perform edit distance clustering (Chance 3)
//
// PARAMETERS:
//
//	prefix - The first token extracted from hostnames (e.g., "api", "web", "app")
//
// RETURNS:
//
//	bool - true if at least one pattern was added, false otherwise
func (p *PatternMiner) processPrefixLevel(prefix string) bool {
	// Step 1: Get all keys starting with this prefix
	prefixKeys := p.getSubdomainsByPrefix(prefix)
	if len(prefixKeys) == 0 {
		return false
	}

	patternAdded := false

	// Step 2: Chance 2 - Generate and store pattern from ALL prefix keys directly
	// Python: r = closure_to_regex(args['target'], keys)
	//         if r not in new_rules and is_good_rule(r, len(keys), ...):
	//           last = prefix  # Only update last when pattern is added
	//           new_rules.add(r)
	if p.tryGenerateAndStorePattern(prefixKeys) {
		patternAdded = true
	}

	// Step 3: Chance 3 - If prefix length > 1, do edit distance clustering
	// Python: if len(prefix) > 1:
	if len(prefix) > 1 {
		// For each k value (distance threshold), compute closures
		for k := p.options.MinLDist; k <= p.options.MaxLDist; k++ {
			// Get clusters by levenshtein distance on prefix keys only
			clusters, err := p.getLevenshteinClustersForKeys(prefixKeys, k)
			if err != nil {
				// Log error but continue processing
				continue
			}

			// For each cluster (closure), generate and store pattern
			// Python: for closure in closures:
			//           r = closure_to_regex(args['target'], closure)
			//           if r not in new_rules and is_good_rule(r, len(closure), ...):
			//             new_rules.add(r)
			for _, cluster := range clusters {
				if p.tryGenerateAndStorePattern(cluster) {
					patternAdded = true
				}
			}
		}
	}

	return patternAdded
}

// getSubdomainsByPrefix returns all subdomains that start with the given prefix.
// Uses radix tree for O(k) lookup where k is the number of matching subdomains.
func (p *PatternMiner) getSubdomainsByPrefix(prefix string) []string {
	var matches []string

	// WalkPrefix traverses all entries in the tree under the given prefix
	p.trie.WalkPrefix(prefix, func(key string, value interface{}) bool {
		matches = append(matches, key)
		return false // continue walking
	})

	return matches
}

// levenshteinSubsClustering clusters subdomains by levenshtein distance on subdomain part.
//
// ALGORITHM (matches Python reference):
// For each distance threshold k:
//   1. Get clusters (edit closures) by levenshtein distance
//   2. For each cluster, generate and store pattern if it passes quality checks
//
// This matches Python:
//   for k in range(args['dist_low'], args['dist_high']):
//     closures = edit_closures(known_hosts, delta=k)
//     for closure in closures:
//       if len(closure) > 1:  # Already filtered in getClustersByLevenshteinDistance
//         r = closure_to_regex(args['target'], closure)
//         if r not in new_rules and is_good_rule(r, len(closure), ...):
//           new_rules.add(r)
func (p *PatternMiner) levenshteinSubsClustering() error {
	// Get clusters by levenshtein distance starting from min to max
	for k := p.options.MinLDist; k <= p.options.MaxLDist; k++ {
		clusters, err := p.getClustersByLevenshteinDistance(k)
		if err != nil {
			return err
		}

		// For each cluster, generate and store pattern
		for _, cluster := range clusters {
			p.tryGenerateAndStorePattern(cluster)
		}
	}
	return nil
}

// getClustersByLevenshteinDistance computes clusters of subdomains bounded by edit distance.
//
// ALGORITHM:
// For each subdomain 'a', create a cluster containing:
//   - The subdomain 'a' itself
//   - All subdomains 'b' where distance(a, b) < k
//
// Then deduplicate identical clusters and discard singletons.
//
// EXAMPLE with k=2:
//
// Given subdomains: api, api1, api12
// Distances: api↔api1=1, api1↔api12=1, api↔api12=2
//
// Step 1: Build cluster for each subdomain
//
//	Cluster from 'api':   {api, api1}      (api1 dist=1 < 2, api12 dist=2 NOT < 2)
//	Cluster from 'api1':  {api1, api, api12}  (api dist=1 < 2, api12 dist=1 < 2)
//	Cluster from 'api12': {api12, api1}    (api1 dist=1 < 2, api dist=2 NOT < 2)
//
// Step 2: Deduplicate (no identical clusters in this case)
//
//	Result: [{api, api1}, {api1, api, api12}, {api12, api1}]
//
// Step 3: Filter singletons (none in this case)
//
//	Final: [{api, api1}, {api1, api, api12}, {api12, api1}]
//
// IMPORTANT PROPERTY:
// Items in a cluster don't need to be close to EACH OTHER, only to the CENTER item.
// In the example above, {api1, api, api12} is a valid cluster even though api↔api12=2 (not < k),
// because both api and api12 are within distance < 2 from the center item api1.
//
// PARAMETERS:
//
//	k - Distance threshold (strictly less than, not <=)
//
// RETURNS:
//
//	Clusters with 2+ items (singletons are discarded)
func (p *PatternMiner) getClustersByLevenshteinDistance(k int) ([][]string, error) {
	if len(p.subdomains) == 0 {
		return nil, nil
	}

	type cluster map[string]struct{}
	var result []cluster

	// For each item 'a', create a cluster containing all items within distance < k from 'a'
	for _, a := range p.subdomains {
		currentCluster := make(cluster)
		currentCluster[a] = struct{}{} // Always include the center item itself

		// Find all items 'b' within distance < k from center item 'a'
		for _, b := range p.subdomains {
			if a == b {
				continue // Already added above
			}

			edge := NewEdge(a, b)
			if dist, ok := p.distanceMap[edge]; ok && dist < k {
				currentCluster[b] = struct{}{}
			}
		}

		// Deduplicate: Check if this exact cluster already exists in results
		found := false
		for _, existingCluster := range result {
			if clustersEqual_internal(currentCluster, existingCluster) {
				found = true
				break
			}
		}

		if !found {
			result = append(result, currentCluster)
		}
	}

	// Convert to slice format and filter out singleton clusters
	finalResult := make([][]string, 0, len(result))
	for _, c := range result {
		if len(c) > 1 {
			items := make([]string, 0, len(c))
			for item := range c {
				items = append(items, item)
			}
			finalResult = append(finalResult, items)
		}
	}

	return finalResult, nil
}

// getLevenshteinClustersForKeys computes levenshtein distance clusters for a specific subset of keys.
// This is similar to getClustersByLevenshteinDistance but operates on a provided subset of keys
// instead of all subdomains.
//
// ALGORITHM:
// For each key 'a' in the provided keys:
//
//	Create a cluster containing:
//	  - The key 'a' itself
//	  - All keys 'b' from the same subset where distance(a, b) < k
//
// PARAMETERS:
//
//	keys - Subset of subdomains to cluster
//	k    - Distance threshold (strictly less than, not <=)
//
// RETURNS:
//
//	Clusters with 2+ items (singletons are discarded)
//
// TODO: Implement levenshtein clustering on subset of keys
func (p *PatternMiner) getLevenshteinClustersForKeys(keys []string, k int) ([][]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	type cluster map[string]struct{}
	var result []cluster

	// For each item 'a' in keys, create a cluster containing all items within distance < k from 'a'
	for _, a := range keys {
		currentCluster := make(cluster)
		currentCluster[a] = struct{}{} // Always include the center item itself

		// Find all items 'b' within distance < k from center item 'a'
		for _, b := range keys {
			if a == b {
				continue // Already added above
			}

			// Look up distance from pre-computed distance map
			edge := NewEdge(a, b)
			if dist, ok := p.distanceMap[edge]; ok && dist < k {
				currentCluster[b] = struct{}{}
			}
		}

		// Deduplicate: Check if this exact cluster already exists in results
		found := false
		for _, existingCluster := range result {
			if clustersEqual_internal(currentCluster, existingCluster) {
				found = true
				break
			}
		}

		if !found {
			result = append(result, currentCluster)
		}
	}

	// Convert to slice format and filter out singleton clusters
	finalResult := make([][]string, 0, len(result))
	for _, c := range result {
		if len(c) > 1 {
			items := make([]string, 0, len(c))
			for item := range c {
				items = append(items, item)
			}
			finalResult = append(finalResult, items)
		}
	}

	return finalResult, nil
}
