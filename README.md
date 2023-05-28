<h1 align="center">
 AlterX
<br>
</h1>


<p align="center">
<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/license-MIT-_red.svg"></a>
<a href="https://goreportcard.com/badge/github.com/projectdiscovery/alterx"><img src="https://goreportcard.com/badge/github.com/projectdiscovery/alterx"></a>
<a href="https://pkg.go.dev/github.com/projectdiscovery/alterx/pkg/alterx"><img src="https://img.shields.io/badge/go-reference-blue"></a>
<a href="https://github.com/projectdiscovery/alterx/releases"><img src="https://img.shields.io/github/release/projectdiscovery/alterx"></a>
<a href="https://twitter.com/pdiscoveryio"><img src="https://img.shields.io/twitter/follow/pdiscoveryio.svg?logo=twitter"></a>
<a href="https://discord.gg/projectdiscovery"><img src="https://img.shields.io/discord/695645237418131507.svg?logo=discord"></a>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#help-menu">Usage</a> •
  <a href="#examples">Running AlterX</a> •
  <a href="https://discord.gg/projectdiscovery">Join Discord</a>

</p>

<pre align="center">
<b>
   Fast and customizable subdomain wordlist generator using DSL.
</b>
</pre>

![image](https://user-images.githubusercontent.com/8293321/229380735-140d3f25-d0cb-461d-8c49-4c1eff43d1f4.png)

## Features
- Fast and Customizable
- **Automatic word enrichment**
- Pre-defined variables
- **Configurable Patterns**
- STDIN / List input

## Installation
To install alterx, you need to have Golang 1.19 installed on your system. You can download Golang from [here](https://go.dev/doc/install). After installing Golang, you can use the following command to install alterx:


```bash
go install github.com/projectdiscovery/alterx/cmd/alterx@latest
```

## Help Menu
You can use the following command to see the available flags and options:

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
   -ms, -max-size int  Max export data size (kb, mb, gb, tb) (default mb)
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

## Why `alterx` ??

what makes `alterx` different from any other subdomain permutation tools like `goaltdns` is its `scripting` feature . alterx takes patterns as input and generates subdomain permutation wordlist based on that pattern similar to what [nuclei](https://github.com/projectdiscovery/nuclei) does with [fuzzing-templates](https://github.com/projectdiscovery/fuzzing-templates) . 

What makes `Active Subdomain Enumeration` difficult is the probability of finding a domain that actually exists. If finding possible subdomains is represented on a scale it should look something like

```console
   Using Wordlist < generate permutations with subdomains (goaltdns) < alterx
```

Almost all popular subdomain permutation tools have hardcoded patterns and when such tools are run they create wordlist which contain subdomains in Millions and this decreases the feasibility of bruteforcing them with tools like dnsx . There is no actual convention to name subdomains and usually depends on person registering the subdomain. with `alterx` it is possible to create patterns based on results from `passive subdomain enumeration` results which increases probability of finding a subdomain and feasibility to bruteforce them.

## Variables

`alterx` uses variable-like syntax similar to nuclei-templates. One can write their own patterns using these variables . when domains are passed as input `alterx` evaluates input and extracts variables from it .

### Basic / Common Variables
  
```yaml
{{sub}}     :  subdomain prefix or left most part of a subdomain
{{suffix}}  :  everything except {{sub}} in subdomain name is suffix
{{tld}}     :  top level domain name (ex com,uk,in etc)
{{etld}}    :  also know as public suffix (ex co.uk , gov.in etc)
```

| Variable   | api.scanme.sh | admin.dev.scanme.sh | cloud.scanme.co.uk |
| ---------- | ------------- | ------------------- | ------------------ |
| `{{sub}}`    | `api`         | `admin`             | `cloud`          |
| `{{suffix}}` | `scanme.sh`   | `dev.scanme.sh`     | `scanme.co.uk`   |
| `{{tld}}`    | `sh`          | `sh`                | `uk`             |
| `{{etld}}`   | `-`           | `-`                 | `co.uk`          |

### Advanced Variables

```yaml
{{root}}  :  also known as eTLD+1 i.e only root domain (ex for api.scanme.sh => {{root}} is scanme.sh)
{{subN}}  :  here N is an integer (ex {{sub1}} , {{sub2}} etc) .

// {{subN}} is advanced variable which exists depending on input
// lets say there is a multi level domain cloud.nuclei.scanme.sh
// in this case {{sub}} = cloud and {{sub1}} = nuclei`
```

| Variable | api.scanme.sh | admin.dev.scanme.sh | cloud.scanme.co.uk |
| -------- | ------------- | ------------------- | ------------------ |
| `{{root}}` | `scanme.sh`   | `scanme.sh`         | `scanme.co.uk`   |
| `{{sub1}}` | `-`           | `dev`               | `-`              |
| `{{sub2}}` | `-`           | `-`                 | `-`              |


## Patterns

pattern in simple terms can be considered as `template` that describes what type of patterns should alterx generate.

```console
// Below are some of example patterns which can be used to generate permutations
// assuming api.scanme.sh was given as input and variable {{word}} was given as input with only one value prod
// alterx generates subdomains for below patterns

"{{sub}}-{{word}}.{{suffix}}" // ex: api-prod.scanme.sh
"{{word}}-{{sub}}.{{suffix}}" // ex: prod-api.scanme.sh
"{{word}}.{{sub}}.{{suffix}}" // ex: prod.api.scanme.sh
"{{sub}}.{{word}}.{{suffix}}" // ex: api.prod.scanme.sh
```

Here is an example pattern config file - https://github.com/projectdiscovery/alterx/blob/main/permutations.yaml that can be easily customizable as per need.

This configuration file generates subdomain permutations for security assessments or penetration tests using customizable patterns and dynamic payloads. Patterns include dash-based, dot-based, and others. Users can create custom payload sections, such as words, region identifiers, or numbers, to suit their specific needs.

For example, a user could define a new payload section `env` with values like `prod` and `dev`, then use it in patterns like `{{env}}-{{word}}.{{suffix}}` to generate subdomains like `prod-app.example.com` and `dev-api.example.com`. This flexibility allows tailored subdomain list for unique testing scenarios and target environments.

Default pattern config file used for generation is stored in `$HOME/.config/alterx/` directory, and custom config file can be also used using `-ac` option.

## Examples

An example of running alterx on existing list of passive subdomains of `tesla.com` yield us **10 additional NEW** and **valid subdomains** resolved using [dnsx](https://github.com/projectdiscovery/dnsx).

```console
$ chaos -d tesla.com | alterx | dnsx

 

   ___   ____          _  __
  / _ | / / /____ ____| |/_/
 / __ |/ / __/ -_) __/>  <  
/_/ |_/_/\__/\__/_/ /_/|_|              

      projectdiscovery.io

[INF] Generated 8312 permutations in 0.0740s
auth-global-stage.tesla.com
auth-stage.tesla.com
digitalassets-stage.tesla.com
errlog-stage.tesla.com
kronos-dev.tesla.com
mfa-stage.tesla.com
paymentrecon-stage.tesla.com
sso-dev.tesla.com
shop-stage.tesla.com
www-uat-dev.tesla.com
```

Similarly `-enrich` option can be used to populate known subdomains as world input to generate **target aware permutations**.

```console
$ chaos -d tesla.com | alterx -enrich

   ___   ____          _  __
  / _ | / / /____ ____| |/_/
 / __ |/ / __/ -_) __/>  <  
/_/ |_/_/\__/\__/_/ /_/|_|              

      projectdiscovery.io

[INF] Generated 662010 permutations in 3.9989s
```

You can alter the default patterns at run time using `-pattern` CLI option. 

```console
$ chaos -d tesla.com | alterx -enrich -p '{{word}}-{{suffix}}'

   ___   ____          _  __
  / _ | / / /____ ____| |/_/
 / __ |/ / __/ -_) __/>  <  
/_/ |_/_/\__/\__/_/ /_/|_|              

      projectdiscovery.io

[INF] Generated 21523 permutations in 0.7984s
```

It is also possible to overwrite existing variables value using `-payload` CLI options.

```console
$ alterx -list tesla.txt -enrich -p '{{word}}-{{year}}.{{suffix}}' -pp word=keywords.txt -pp year=2023

   ___   ____          _  __
  / _ | / / /____ ____| |/_/
 / __ |/ / __/ -_) __/>  <  
/_/ |_/_/\__/\__/_/ /_/|_|              

      projectdiscovery.io

[INF] Generated 21419 permutations in 1.1699s
```

**For more information, please checkout the release blog** - https://blog.projectdiscovery.io/introducing-alterx-simplifying-active-subdomain-enumeration-with-patterns/


Do also check out the below similar open-source projects that may fit in your workflow:

[altdns](https://github.com/infosec-au/altdns), [goaltdns](https://github.com/subfinder/goaltdns), [gotator](https://github.com/Josue87/gotator), [ripgen](https://github.com/resyncgg/ripgen/), [dnsgen](https://github.com/ProjectAnte/dnsgen), [dmut](https://github.com/bp0lr/dmut), [permdns](https://github.com/hpy/permDNS), [str-replace](https://github.com/j3ssie/str-replace), [dnscewl](https://github.com/codingo/DNSCewl), [regulator](https://github.com/cramppet/regulator)


--------

<div align="center">

**alterx** is made with ❤️ by the [projectdiscovery](https://projectdiscovery.io) team and distributed under [MIT License](LICENSE.md).


<a href="https://discord.gg/projectdiscovery"><img src="https://raw.githubusercontent.com/projectdiscovery/nuclei-burp-plugin/main/static/join-discord.png" width="300" alt="Join Discord"></a>

</div>