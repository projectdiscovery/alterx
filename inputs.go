package alterx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/projectdiscovery/gologger"
	urlutil "github.com/projectdiscovery/utils/url"
	"golang.org/x/net/publicsuffix"
)

// Input contains parsed/evaluated data of a URL
type Input struct {
	TLD        string   // only TLD (right most part of subdomain) ex: `.uk`
	ETLD       string   // Simply put public suffix (ex: co.uk)
	SLD        string   // Second-level domain (ex: scanme)
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
		"sld":    i.SLD,
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
func NewInput(inputURL string) (*Input, error) {
	URL, err := urlutil.Parse(inputURL)
	if err != nil {
		return nil, err
	}
	// check if hostname contains *
	if strings.Contains(URL.Hostname(), "*") {
		if strings.HasPrefix(URL.Hostname(), "*.") {
			tmp := strings.TrimPrefix(URL.Hostname(), "*.")
			URL.Host = strings.Replace(URL.Host, URL.Hostname(), tmp, 1)
		}
		// if * is present in middle ex: prod.*.hackerone.com
		// skip it
		if strings.Contains(URL.Hostname(), "*") {
			return nil, fmt.Errorf("input %v is not a valid url , skipping", inputURL)
		}
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
		// this happens if input domain does not have eTLD+1 at all ex: `.com` or `co.uk`
		gologger.Warning().Msgf("input domain %v is eTLD/publicsuffix and not a valid domain name", URL.Hostname())
		return ivar, nil
	}
	ivar.Root = rootDomain
	if ivar.ETLD != "" {
		ivar.SLD = strings.TrimSuffix(rootDomain, "."+ivar.ETLD)
	} else {
		ivar.SLD = strings.TrimSuffix(rootDomain, "."+ivar.TLD)
	}
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
