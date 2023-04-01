# alterx

Fast and customizable subdomain wordlist generator using DSL


## Why `alterx` ??

what makes `alterx` different from any other subdomain permutation tools like `goaltdns` is its `scripting` feature . alterx takes patterns as input and generates subdomain permutation wordlist based on that pattern similar to what `nuclei` does with `fuzzing-templates` . 

What makes `Active Subdomain Enumeration` difficult is the probability of finding a domain that actually exists . If finding possible subdomains is represented on a scale it should look something like

```console
   Using Wordlist < generate permutations with subdomains (goaltdns) < alterx
```

Almost all popular subdomain permutation tools have hardcoded patterns and when such tools are run they create wordlist which contain subdomains in Millions and this decreases the feasibility of bruteforcing them with tools like dnsx . There is no actual convention to name subdomains and usually depends on person registering the subdomain. with `alterx` it is possible to create patterns based on results from `passive subdomain enumeration` results which increases probability of finding a subdomain and feasibility to bruteforce them



## Variables

`alterx` uses variable-like syntax similar to nuclei-templates. One can write their own patterns using these variables . when domains are passed as input `alterx` evaluates input and extracts variables from it .

### Basic / Common Variables
  
~~~yaml
{{sub}}    :  subdomain prefix or left most part of a subdomain
{{suffix}} :  everything except {{sub}} in subdomain name is suffix
{{tld}}    :  top level domain name (ex com,uk,in etc)
{{etld}}   :  also know as public suffix (ex co.uk , gov.in etc)
~~~

| Variable   | api.scanme.sh | admin.dev.scanme.sh | cloud.scanme.co.uk |
| ---------- | ------------- | ------------------- | ------------------ |
| {{sub}}    | `api`         | `admin`             | `cloud`            |
| {{suffix}} | `scanme.sh`   | `dev.scanme.sh`     | `scanme.co.uk`     |
| {{tld}}    | `sh`          | `sh`                | `uk`               |
| {{etld}}   | `-`           | `-`                 | `co.uk`            |

### Advanced Variables

~~~yaml
{{root}}  :  also known as eTLD+1 i.e only root domain (ex for `api.scanme.sh` => {{root}} is `scanme.sh`)
{{subN}}  :  here N is an integer (ex {{sub1}} , {{sub2}} etc) .

// {{subN}} is advanced variable which exists depending on input
// lets say there is a multi level domain `cloud.nuclei.scanme.sh`
// in this case `{{sub}} = cloud` and `{{sub1}} = nuclei`
~~~

| Variable | api.scanme.sh | admin.dev.scanme.sh | cloud.scanme.co.uk |
| -------- | ------------- | ------------------- | ------------------ |
| {{root}} | `scanme.sh`   | `scanme.sh`         | `scanme.co.uk`     |
| {{sub1}} | `-`           | `dev`               | `-`                |
| {{sub2}} | `-`           | `-`                 | `-`                |



## Patterns

pattern in simple terms can be considered as `template` that describes what type of patterns should alterx generate

```console
// Below are some of example patterns which can be used to generate permutations
// assuming `api.scanme.sh` was given as input and variable {{word}} was given as input with only one value `prod`
// alterx generates subdomains for below patterns

"{{sub}}-{{word}}.{{suffix}}" // ex: api-prod.scanme.sh
"{{word}}-{{sub}}.{{suffix}}" // ex: prod-api.scanme.sh
"{{word}}.{{sub}}.{{suffix}}" // ex: prod.api.scanme.sh
"{{sub}}.{{word}}.{{suffix}}" // ex: api.prod.scanme.sh
```

## Usage

```console
Fast and customizable subdomain wordlist generator using DSL.

Usage:
  ./alterx [flags]

Flags:
INPUT:
   -l, -list string[]     subdomains to use when creating permutations (stdin, comma-separated, file)
   -p, -pattern string[]  custom permutation patterns input to generate (comma-seperated, file)
   -pp, -payload value    custom payload pattern input to replace/use in key=value format (-pp 'word=words.txt')

OUTPUT:
   -es, -estimate      estimate permutation count without generating payloads
   -o, -output string  output file to write altered subdomain list
   -v, -verbose        display verbose output
   -silent             display results only
   -version            display alterx version

CONFIG:
   -config string  alterx cli config file (default '$HOME/.config/alterx/config.yaml')
   -en, -enrich    enrich wordlist by extracting words from input
   -ac string      alterx permutation config file (default '$HOME/.config/alterx/permutation_v0.0.1.yaml')
   -limit int      limit the number of results to return (default 0)

UPDATE:
   -up, -update                 update alterx to latest version
   -duc, -disable-update-check  disable automatic alterx update check
```

```console
$ ./alterx -list passive.txt
api-dev.scanme.sh
api-lib.scanme.sh
api-prod.scanme.sh
api-stage.scanme.sh
api-wp.scanme.sh
dev-api.scanme.sh
lib-api.scanme.sh
prod-api.scanme.sh
stage-api.scanme.sh
.....
```