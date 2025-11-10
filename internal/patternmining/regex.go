package patternmining

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// tokenize breaks down hostnames into structured tokens
func (m *Miner) tokenize(items []string) [][][]string {
	var ret [][][]string
	for _, item := range items {
		subdomain := strings.TrimSuffix(item, "."+m.opts.Target)
		labelsStr := strings.Split(subdomain, ".")
		var n [][]string
		for _, labelStr := range labelsStr {
			var t []string
			parts := strings.Split(labelStr, "-")
			var tokens []string
			for i, p := range parts {
				if i == 0 {
					tokens = append(tokens, p)
				} else {
					tokens = append(tokens, "-"+p)
				}
			}
			for _, token := range tokens {
				subt := splitNum(token)
				var tt []string
				for ii := 0; ii < len(subt); ii++ {
					if subt[ii] == "-" && ii+1 < len(subt) {
						subt[ii+1] = "-" + subt[ii+1]
					} else {
						tt = append(tt, subt[ii])
					}
				}
				t = append(t, tt...)
			}
			n = append(n, t)
		}
		ret = append(ret, n)
	}
	return ret
}

// splitNum splits strings by numeric sequences
func splitNum(s string) []string {
	loc := reNum.FindAllStringIndex(s, -1)
	var res []string
	start := 0
	for _, l := range loc {
		if l[0] > start {
			res = append(res, s[start:l[0]])
		}
		res = append(res, s[l[0]:l[1]])
		start = l[1]
	}
	if start < len(s) {
		res = append(res, s[start:])
	}
	var ne []string
	for _, p := range res {
		if p != "" {
			ne = append(ne, p)
		}
	}
	return ne
}

// closureToRegex generates a regex pattern from a cluster of similar hosts
func (m *Miner) closureToRegex(escaped bool, members []string) (string, int64) {
	if len(members) == 0 {
		return "", 0
	}
	tokens := m.tokenize(members)
	var maxLevel int
	for _, memTokens := range tokens {
		for i := range memTokens {
			if i > maxLevel {
				maxLevel = i
			}
		}
	}
	optional := make(map[int]map[int][]string)
	levels := make(map[int]map[int]map[string]bool)
	for _, memTokens := range tokens {
		for i := range memTokens {
			if _, ok := optional[i]; !ok {
				optional[i] = make(map[int][]string)
			}
			if _, ok := levels[i]; !ok {
				levels[i] = make(map[int]map[string]bool)
			}
			for j := 0; j < len(memTokens[i]); j++ {
				optional[i][j] = append(optional[i][j], memTokens[i][j])
			}
			for j, token := range memTokens[i] {
				if _, ok := levels[i][j]; !ok {
					levels[i][j] = make(map[string]bool)
				}
				levels[i][j][token] = true
			}
		}
	}
	numMembers := len(members)
	var ret strings.Builder
	for i := 0; i <= maxLevel; i++ {
		if _, ok := levels[i]; !ok {
			continue
		}
		var poss []int
		for j := range levels[i] {
			poss = append(poss, j)
		}
		sort.Ints(poss)
		isLevel0 := i == 0
		var n strings.Builder
		if !isLevel0 {
			n.WriteString("(.")
		}
		for _, j := range poss {
			var toks []string
			for tk := range levels[i][j] {
				toks = append(toks, tk)
			}
			sort.Strings(toks)
			var alt string
			if len(toks) == 0 {
				continue
			}
			alt = strings.Join(toks, "|")
			isOptPos := len(optional[i][j]) != numMembers
			if isLevel0 && j == 0 {
				n.WriteString("(" + alt + ")")
			} else if len(toks) == 1 && j == 0 {
				n.WriteString(alt)
			} else {
				q := ""
				if isOptPos {
					q = "?"
				}
				n.WriteString("(" + alt + ")" + q)
			}
		}
		// Calculate optional level
		var posLists [][]string
		for _, j := range poss {
			posLists = append(posLists, optional[i][j])
		}
		minLen := numMembers
		for _, pl := range posLists {
			if len(pl) < minLen {
				minLen = len(pl)
			}
		}
		valueSet := make(map[string]bool)
		for kk := 0; kk < minLen; kk++ {
			var sb strings.Builder
			for _, pl := range posLists {
				if kk < len(pl) {
					sb.WriteString(pl[kk])
				}
			}
			valueSet[sb.String()] = true
		}
		isOptionalLevel := len(valueSet) != 1 || minLen != numMembers
		if isLevel0 {
			ret.WriteString(n.String())
		} else {
			ret.WriteString(n.String())
			if isOptionalLevel {
				ret.WriteString(")?")
			} else {
				ret.WriteString(")")
			}
		}
	}
	var full strings.Builder
	full.WriteString(ret.String())
	if escaped {
		full.WriteString(`\.`)
		full.WriteString(escapeLiteral(m.opts.Target))
	} else {
		full.WriteString(".")
		full.WriteString(m.opts.Target)
	}
	r := full.String()
	compressed := m.compressNumberRanges(r)
	return compressed, 0
}

// escapeLiteral escapes regex special characters
func escapeLiteral(s string) string {
	var sb strings.Builder
	for _, c := range s {
		switch c {
		case '.', '^', '$', '*', '+', '?', '(', ')', '[', '{', '}', '|', '\\':
			sb.WriteRune('\\')
			fallthrough
		default:
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

// compressNumberRanges optimizes number sequences in regex patterns
func (m *Miner) compressNumberRanges(regex string) string {
	repl := make(map[string]string)
	extraM := make(map[string]string)
	hyphenM := make(map[string]bool)
	var stack []int
	i := 0
	for i < len(regex) {
		if regex[i] == '(' {
			stack = append(stack, i)
			i++
			continue
		}
		if regex[i] == ')' && len(stack) > 0 {
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			group := regex[start+1 : i]
			if strings.ContainsAny(group, "(?)") {
				i++
				continue
			}
			tokens := strings.Split(group, "|")
			var numbers []string
			var nonnumbers []string
			var hyphenated []string
			for _, token := range tokens {
				token = strings.TrimSpace(token)
				if token == "" {
					continue
				}
				if reNumeric.MatchString(token) {
					numbers = append(numbers, token)
				} else if strings.HasPrefix(token, "-") && reNumeric.MatchString(token[1:]) {
					hyphenated = append(hyphenated, token[1:])
				} else {
					nonnumbers = append(nonnumbers, token)
				}
			}
			if len(numbers) > 0 && len(hyphenated) > 0 {
				i++
				continue
			}
			if len(numbers) <= 1 && len(hyphenated) <= 1 {
				i++
				continue
			}
			g1 := ""
			g2 := strings.Join(nonnumbers, "|")
			if len(numbers) > 1 {
				g1 = strings.Join(numbers, "|")
			} else {
				g1 = strings.Join(hyphenated, "|")
			}
			fullGroup := "(" + group + ")"
			repl[g1] = fullGroup
			extraM[g1] = g2
			hyphenM[g1] = len(hyphenated) > 1
			i++
			continue
		}
		i++
	}
	ret := regex
	var keys []string
	for k := range repl {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, group := range keys {
		generalized := "("
		if hyphenM[group] {
			generalized = "(-"
		}
		positions := make(map[int]map[int]bool)
		toks := strings.Split(group, "|")
		var revToks []string
		for _, g := range toks {
			rs := reverseString(g)
			revToks = append(revToks, rs)
		}
		for _, token := range revToks {
			for position := 0; position < len(token); position++ {
				symbolStr := string(token[position])
				symbol, _ := strconv.Atoi(symbolStr)
				if _, ok := positions[position]; !ok {
					positions[position] = make(map[int]bool)
				}
				positions[position][symbol] = true
			}
		}
		s := revToks
		sort.Slice(s, func(p, q int) bool { return len(s[p]) < len(s[q]) })
		start := len(s[len(s)-1]) - 1
		end := len(s[0]) - 1
		for ii := start; ii > end; ii-- {
			if _, ok := positions[ii]; !ok {
				positions[ii] = make(map[int]bool)
			}
			positions[ii][-1] = true
		}
		var possPos []int
		for p := range positions {
			possPos = append(possPos, p)
		}
		sort.Ints(possPos)
		for kk := len(possPos) - 1; kk >= 0; kk-- {
			pos := possPos[kk]
			symbolsMap := positions[pos]
			hasNone := symbolsMap[-1]
			delete(symbolsMap, -1)
			var symbols []int
			for k := range symbolsMap {
				symbols = append(symbols, k)
			}
			sort.Ints(symbols)
			if len(symbols) == 0 {
				continue
			}
			startS := symbols[0]
			endS := symbols[len(symbols)-1]
			if startS == endS {
				generalized += strconv.Itoa(startS)
			} else {
				generalized += fmt.Sprintf("[%d-%d]", startS, endS)
			}
			if hasNone {
				generalized += "?"
			}
		}
		generalized += ")"
		ext := extraM[group]
		if ext != "" {
			generalized = "(" + generalized + "|(" + ext + "))"
		}
		rep := repl[group]
		ret = strings.ReplaceAll(ret, rep, generalized)
	}
	return ret
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

