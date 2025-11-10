package alterx

import (
	"fmt"
	"regexp"
	"strings"
	"unsafe"

	"golang.org/x/net/publicsuffix"
)

var varRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9]+)\}\}`)

// returns no of variables present in statement
func getVarCount(data string) int {
	return len(varRegex.FindAllStringSubmatch(data, -1))
}

// returns names of all variables
func getAllVars(data string) []string {
	var values []string
	for _, v := range varRegex.FindAllStringSubmatch(data, -1) {
		if len(v) >= 2 {
			values = append(values, v[1])
		}
	}
	return values
}

// getSampleMap returns a sample map containing input variables and payload variable
func getSampleMap(inputVars map[string]interface{}, payloadVars map[string][]string) map[string]interface{} {
	sMap := map[string]interface{}{}
	for k, v := range inputVars {
		sMap[k] = v
	}
	for k, v := range payloadVars {
		if k != "" && len(v) > 0 {
			sMap[k] = "temp"
		}
	}
	return sMap
}

// checkMissing checks if all variables/placeholders are successfully replaced
// if not error is thrown with description
func checkMissing(template string, data map[string]interface{}) error {
	got := Replace(template, data)
	if res := varRegex.FindAllString(got, -1); len(res) > 0 {
		return fmt.Errorf("values of `%v` variables not found", strings.Join(res, ","))
	}
	return nil
}

// TODO: add this to utils
// unsafeToBytes converts a string to byte slice and does it with
// zero allocations.
//
// Reference - https://stackoverflow.com/questions/59209493/how-to-use-unsafe-get-a-byte-slice-from-a-string-without-memory-copy
func unsafeToBytes(data string) []byte {
	return unsafe.Slice(unsafe.StringData(data), len(data))
}

func getNValidateRootDomain(domains []string) (string, error) {
	if len(domains) == 0 {
		return "", fmt.Errorf("no domains provided")
	}

	var rootDomain string
	// parse root domain from publicsuffix for first entry
	for _, domain := range domains {
		if strings.TrimSpace(domain) == "" {
			continue
		}
		if rootDomain == "" {
			root, err := publicsuffix.EffectiveTLDPlusOne(domain)
			if err != nil || root == "" {
				return "", fmt.Errorf("failed to derive root domain from %v: %v", domain, err)
			}
			rootDomain = root
		} else {
			if domain != rootDomain && !strings.HasSuffix(domain, "."+rootDomain) {
				return "", fmt.Errorf("domain %v does not have the same root domain as %v, only homogeneous domains are supported in discover mode", domain, rootDomain)
			}
		}
	}
	if rootDomain == "" {
		return "", fmt.Errorf("no valid domains found after filtering empty entries")
	}
	return rootDomain, nil
}
