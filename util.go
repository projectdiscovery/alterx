package alterx

import (
	"regexp"
	"strings"

	errorutil "github.com/projectdiscovery/utils/errors"
)

var varRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9]+)\}\}`)

// returns no of variables present in statement
func getVarCount(data string) int {
	return len(varRegex.FindAllStringSubmatch(data, -1))
}

// returns names of all variables
func getAllVars(data string) []string {
	values := []string{}
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
		return errorutil.NewWithTag("alterx", "missing `%v` variables", strings.Join(res, ","))
	}
	return nil
}
