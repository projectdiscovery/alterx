package alterx

import (
	"fmt"

	"github.com/projectdiscovery/fasttemplate"
)

const (
	// General marker (open/close)
	General = "§"
	// ParenthesisOpen marker - begin of a placeholder
	ParenthesisOpen = "{{"
	// ParenthesisClose marker - end of a placeholder
	ParenthesisClose = "}}"
)

// Replace replaces placeholders in template with values on the fly.
func Replace(template string, values map[string]interface{}) string {
	valuesMap := make(map[string]interface{}, len(values))
	for k, v := range values {
		valuesMap[k] = fmt.Sprint(v)
	}
	replaced := fasttemplate.ExecuteStringStd(template, ParenthesisOpen, ParenthesisClose, valuesMap)
	final := fasttemplate.ExecuteStringStd(replaced, General, General, valuesMap)
	return final
}
