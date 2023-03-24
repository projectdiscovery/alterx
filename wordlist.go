package alterx

var DefaultWordList = map[string][]string{
	"word": {
		"dev", "lib", "prod", "stage", "wp",
	},
}

var DefaultPatterns = []string{
	"{{sub}}-{{word}}.{{suffix}}", // ex: api-prod.scanme.sh
	"{{word}}-{{sub}}.{{suffix}}", // ex: prod-api.scanme.sh
	"{{word}}.{{sub}}.{{suffix}}", // ex: prod.api.scanme.sh
	"{{sub}}.{{word}}.{{suffix}}", // ex: api.prod.scanme.sh
}
