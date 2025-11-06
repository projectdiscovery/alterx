package mining

/// Jargons & Definitions
/// for api5.dev.example.com
// subdomain/subdomainpart = api5.dev
// root/ root domain = example.com
// level0 = api5
// level1 = dev
// for level 0 , section1 = api , section2 = 5

import (
	"strings"

	"github.com/armon/go-radix"
	levenshtein "github.com/ka-weihe/fast-levenshtein"
	"github.com/projectdiscovery/utils/errkit"
	mapsutil "github.com/projectdiscovery/utils/maps"
	"golang.org/x/net/publicsuffix"
)

var (
	ErrNoDomains = errkit.New("no domains provided to mine patterns")
)

type Options struct {
	// MinLDist is the minimum levenshtein distance for clustering
	MinLDist int
	// MaxLDist is the maximum levenshtein distance for clustering
	MaxLDist int
	// PatternThreshold is the threshold after which pattern will be discarded
	// because of being too generic
	PatternThreshold float64
	// PatternQualityRatio is the ratio of output/input patterns
	// after generating patterns from a cluster it is used to discard low quality patterns
	// whose ratio is greater than this threshold
	PatternQualityRatio float64
	// MaxPatternLength is the maximum length of generated pattern string
	// patterns exceeding this length are discarded
	MaxPatternLength int
}

func (o *Options) applyDefaults() {
	// reference from regulator
	if o.MinLDist == 0 {
		o.MinLDist = 2
	}
	if o.MaxLDist == 0 {
		o.MaxLDist = 10
	}
}

// PatternMiner is the main struct for pattern mining
// it mines for patterns for the given domains
type PatternMiner struct {
	rootDomains    []string
	subdomains     []string
	trie           *radix.Tree            // radix tree for fast prefix searches
	distanceMap    map[Edge]int           // contains distance betwen two nodes or items
	options        *Options
	results        []*DSLPattern          // collected patterns that passed quality checks
	seenPatterns   map[string]struct{}    // deduplication: tracks pattern strings already generated
}

// NewPatternMiner creates a new pattern miner instance
func NewPatternMiner(domains []string, opts *Options) (*PatternMiner, error) {
	if len(domains) == 0 {
		return nil, ErrNoDomains
	}
	opts.applyDefaults()
	p := &PatternMiner{
		distanceMap:  make(map[Edge]int),
		options:      opts,
		trie:         radix.New(),
		results:      make([]*DSLPattern, 0),
		seenPatterns: make(map[string]struct{}),
	}
	if err := p.prepare(domains); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *PatternMiner) prepare(domains []string) error {
	var subs = make(map[string]struct{})
	var rootDomains = make(map[string]struct{})
	for _, domain := range domains {
		rootDomain, err := publicsuffix.EffectiveTLDPlusOne(domain)
		if err != nil {
			return err
		}
		if _, ok := rootDomains[rootDomain]; !ok {
			rootDomains[rootDomain] = struct{}{}
		}
		sub := strings.TrimSuffix(domain, "."+rootDomain)
		if _, ok := subs[sub]; !ok {
			subs[sub] = struct{}{}
		}
	}
	p.rootDomains = mapsutil.GetKeys(rootDomains)
	p.subdomains = mapsutil.GetKeys(subs)

	// build radix tree for fast prefix searches
	// this tree is used to do fast lookup of all subdomains with a given prefix
	// ex: prefix "ap" will return api, api1, app, etc.
	for k := range subs {
		p.trie.Insert(k, nil) // value is nil since we only need to track keys
	}

	// distance map
	// calculate levenshtein distance between all subdomains
	// ex: distance between api and api1 is 1
	// while distance between api and apple is 3
	for _, x := range p.subdomains {
		for _, y := range p.subdomains {
			if x == y {
				continue
			}
			// get a predicatable edgename between subdomains
			edge := NewEdge(x, y)
			if _, ok := p.distanceMap[edge]; !ok {
				p.distanceMap[edge] = levenshtein.Distance(x, y)
			}
		}
	}
	return nil
}

// GetResults returns all patterns that were generated and passed quality checks.
// This should be called after Execute() completes.
func (p *PatternMiner) GetResults() []*DSLPattern {
	return p.results
}

// Execute mines for patterns from all existing data
func (p *PatternMiner) Execute() error {
	// The core idea of the algorithm is to group or cluster subdomains
	// into a set of unique subdomains that might be related in some way
	// for each such group execute GeneratePattern() method to generate patterns
	// from that group

	// to generate high quality patterns we cluster subdomains using many
	// clustering approaches and generate patterns from each group and combine them
	// when generating pattern, we purge low quality patterns by using a ratio of input/output patterns

	// Approaches used

	// 1) Levenshtein Distance on Subdomain Part Clustering
	if err := p.levenshteinSubsClustering(); err != nil {
		return err
	}

	// 2) Hierarchical Ngram-Based Clustering
	// This approach uses a multi-level hierarchy that combines:
	//   - Unigram/Bigram Prefix Clustering
	//   - Full Token Prefix Matching (extract first token, then cluster by that prefix)
	//   - Levenshtein Distance on Prefixes Clustering (edit distance on prefix-matched subsets)
	//
	// Flow: ngram → keys → generate pattern (Chance 1)
	//            → extract prefixes → for each prefix:
	//                               → get keys → generate pattern (Chance 2)
	//                                         → edit distance clustering → patterns (Chance 3)
	//
	// This hierarchical approach provides multiple chances to generate patterns at different
	// levels of granularity, resulting in comprehensive pattern mining.
	if err := p.hierarchicalNgramClustering(); err != nil {
		return err
	}

	return nil
}
