package alterx

import (
	"strconv"
	"strings"

	errorutil "github.com/projectdiscovery/utils/errors"
	urlutil "github.com/projectdiscovery/utils/url"
	"golang.org/x/net/publicsuffix"
)

// Input contains parsed/evaluated data of a URL
type Input struct {
	TLD        string   // only TLD (right most part of subdomain) ex: `.uk`
	ETLD       string   // Simply put public suffix (ex: co.uk)
	Root       string   // Root Domain (eTLD+1) of Subdomain
	Sub        string   // Sub or LeftMost prefix of subdomain
	Suffix     string   // suffix is everything except `Sub` (Note: if domain is not multilevel Suffix==Root)
	MultiLevel []string // (Optional) store prefix of multi level subdomains
}

// GetMap returns variables map of input
func (i *Input) GetMap() map[string]interface{} {
	m := map[string]interface{}{
		"tld":    i.TLD,
		"etld":   i.ETLD,
		"root":   i.Root,
		"sub":    i.Sub,
		"suffix": i.Suffix,
	}
	for k, v := range i.MultiLevel {
		m["sub"+strconv.Itoa(k+1)] = v
	}
	for k, v := range m {
		if v == "" {
			// purge empty vars
			delete(m, k)
		}
	}
	return m
}

// NewInput parses URL to Input Vars
func NewInput(inpurURL string) (*Input, error) {
	URL, err := urlutil.Parse(inpurURL)
	if err != nil {
		return nil, err
	}
	ivar := &Input{}
	suffix, _ := publicsuffix.PublicSuffix(URL.Hostname())
	if strings.Contains(suffix, ".") {
		ivar.ETLD = suffix
		arr := strings.Split(suffix, ".")
		ivar.TLD = arr[len(arr)-1]
	} else {
		ivar.TLD = suffix
	}
	rootDomain, err := publicsuffix.EffectiveTLDPlusOne(URL.Hostname())
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("failed to extra eTLD+1 for %v", URL.Hostname())
	}
	ivar.Root = rootDomain
	// anything before root domain is subdomain
	subdomainPrefix := strings.TrimSuffix(URL.Hostname(), rootDomain)
	subdomainPrefix = strings.TrimSuffix(subdomainPrefix, ".")
	if strings.Contains(subdomainPrefix, ".") {
		// this is a multi level subdomain
		// ex: something.level.scanme.sh
		// in such cases variable name starts after 1st prefix
		prefixes := strings.Split(subdomainPrefix, ".")
		ivar.Sub = prefixes[0]
		ivar.MultiLevel = prefixes[1:]
	} else {
		ivar.Sub = subdomainPrefix
	}
	ivar.Suffix = strings.TrimPrefix(URL.Hostname(), ivar.Sub+".")
	return ivar, nil
}
