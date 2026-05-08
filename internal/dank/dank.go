package dank

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DankEncoder implementation matching Python's C++ backend exactly
// Uses Brzozowski's algorithm for DFA minimization

// preprocessRegex expands character classes like [1-5] to (1|2|3|4|5)
func preprocessRegex(regex string) string {
	// Find all character classes of form [x-y]
	re := regexp.MustCompile(`\[(.)\-(.)\]`)

	for {
		match := re.FindStringSubmatchIndex(regex)
		if match == nil {
			break
		}

		// Extract start and end characters
		startChar := regex[match[2]]
		endChar := regex[match[4]]

		// Expand range
		var elements []string
		for ch := startChar; ch <= endChar; ch++ {
			elements = append(elements, string(ch))
		}
		expanded := "(" + strings.Join(elements, "|") + ")"

		// Replace in regex
		fullMatch := regex[match[0]:match[1]]
		regex = strings.Replace(regex, fullMatch, expanded, 1)
	}

	return regex
}

// NFAState represents a state in the Thompson NFA (C++ style).
type NFAState struct {
	ID      int
	Trans   map[byte]map[int]bool // byte -> set of state IDs
	IsFinal bool
}

// DFAState represents a DFA state (subset of NFA states).
type DFAState struct {
	ID      int
	NFAIDs  []int // sorted for key
	Trans   map[byte]int
	IsFinal bool
}

// DankEncoder is the main struct matching Python's C++ backend.
type DankEncoder struct {
	regex        string
	alphabet     []byte
	nfa          []*NFAState
	initStates   map[int]bool
	dfaStates    map[string]int
	dfa          []*DFAState
	fixedSlice   int
	stateCounter int
}

// NewDankEncoder initializes and builds the automaton using C++ algorithm.
func NewDankEncoder(regexStr string, fixedSlice int) *DankEncoder {
	dnsAlphabet := []byte("abcdefghijklmnopqrstuvwxyz0123456789._-")

	// Preprocess regex to expand character classes (like Python's preprocess)
	preprocessed := preprocessRegex(regexStr)

	d := &DankEncoder{
		regex:        preprocessed,
		alphabet:     dnsAlphabet,
		fixedSlice:   fixedSlice,
		stateCounter: 0,
	}

	// Build NFA using C++ algorithm
	d.buildNFAFromRegex(preprocessed)

	// Build DFA using Brzozowski's algorithm (like Python's C++)
	// determinize -> reverse -> determinize -> reverse -> determinize
	d.buildDFA()
	d.reverseDFA()
	d.buildDFA()
	d.reverseDFA()
	d.buildDFA()

	return d
}

// buildNFAFromRegex constructs NFA matching C++ from_regex
func (d *DankEncoder) buildNFAFromRegex(regex string) {
	// Initialize: create states 0 and 1, state 1 is final
	d.nfa = []*NFAState{
		{ID: 0, Trans: make(map[byte]map[int]bool)},
		{ID: 1, Trans: make(map[byte]map[int]bool), IsFinal: true},
	}
	d.stateCounter = 2

	// Build from start=0 to end=1
	d.fromRegex(0, 1, regex)

	// Get epsilon closure of initial state
	d.initStates = make(map[int]bool)
	d.initStates[0] = true
	d.epsilonClosure(d.initStates)
}

// fromRegex implements _from_regex from C++
func (d *DankEncoder) fromRegex(s, t int, pattern string) {
	if len(pattern) == 0 {
		d.insertNFA(s, 0, t) // epsilon transition
		return
	}

	// Single character (including escaped)
	if len(pattern) == 1 {
		d.insertNFA(s, pattern[0], t)
		return
	}

	// Escaped character
	if len(pattern) == 2 && pattern[0] == '\\' {
		d.insertNFA(s, pattern[1], t)
		return
	}

	// Find rightmost top-level | or concatenation point
	optionPos := -1
	concatPos := -1
	depth := 0

	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]

		switch ch {
		case '\\':
			if depth == 0 {
				concatPos = i
			}
			i++ // Skip next char
		case '(':
			if depth == 0 {
				concatPos = i
			}
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				optionPos = i
			}
		case '?', '*', '+':
			// Don't update concat for operators
		default:
			if depth == 0 {
				concatPos = i
			}
		}
	}

	// Handle alternation (|)
	if optionPos >= 0 {
		// Create intermediate states
		i0 := d.newState()
		i1 := d.newState()
		d.insertNFA(s, 0, i0) // epsilon
		d.insertNFA(i1, 0, t) // epsilon
		d.fromRegex(i0, i1, pattern[:optionPos])

		i0 = d.newState()
		i1 = d.newState()
		d.insertNFA(s, 0, i0) // epsilon
		d.insertNFA(i1, 0, t) // epsilon
		d.fromRegex(i0, i1, pattern[optionPos+1:])
		return
	}

	// Handle concatenation
	if concatPos > 0 {
		i0 := d.newState()
		i1 := d.newState()
		d.insertNFA(i0, 0, i1) // epsilon
		d.fromRegex(s, i0, pattern[:concatPos])
		d.fromRegex(i1, t, pattern[concatPos:])
		return
	}

	// Handle postfix operators
	lastChar := pattern[len(pattern)-1]

	if lastChar == '?' {
		i0 := d.newState()
		i1 := d.newState()
		d.insertNFA(s, 0, i0) // epsilon
		d.insertNFA(s, 0, t)  // epsilon (skip)
		d.insertNFA(i1, 0, t) // epsilon
		d.fromRegex(i0, i1, pattern[:len(pattern)-1])
		return
	}

	if lastChar == '*' {
		i0 := d.newState()
		i1 := d.newState()
		d.insertNFA(s, 0, i0)  // epsilon
		d.insertNFA(s, 0, t)   // epsilon (skip)
		d.insertNFA(i1, 0, i0) // epsilon (loop)
		d.insertNFA(i1, 0, t)  // epsilon (exit)
		d.fromRegex(i0, i1, pattern[:len(pattern)-1])
		return
	}

	if lastChar == '+' {
		i0 := d.newState()
		i1 := d.newState()
		d.insertNFA(i0, 0, i1) // epsilon
		d.fromRegex(s, i0, pattern[:len(pattern)-1])

		s = i1
		i0 = d.newState()
		i1 = d.newState()
		d.insertNFA(s, 0, i0)  // epsilon
		d.insertNFA(s, 0, t)   // epsilon (skip)
		d.insertNFA(i1, 0, i0) // epsilon (loop)
		d.insertNFA(i1, 0, t)  // epsilon (exit)
		d.fromRegex(i0, i1, pattern[:len(pattern)-1])
		return
	}

	// Must be wrapped in parentheses
	if pattern[0] == '(' && pattern[len(pattern)-1] == ')' {
		d.fromRegex(s, t, pattern[1:len(pattern)-1])
		return
	}

	// Shouldn't reach here
	panic(fmt.Sprintf("Unexpected pattern: %s", pattern))
}

// newState creates a new NFA state
func (d *DankEncoder) newState() int {
	id := d.stateCounter
	d.stateCounter++
	d.nfa = append(d.nfa, &NFAState{
		ID:    id,
		Trans: make(map[byte]map[int]bool),
	})
	return id
}

// insertNFA adds a transition
func (d *DankEncoder) insertNFA(from int, ch byte, to int) {
	if d.nfa[from].Trans[ch] == nil {
		d.nfa[from].Trans[ch] = make(map[int]bool)
	}
	d.nfa[from].Trans[ch][to] = true
}

// epsilonClosure computes epsilon closure
func (d *DankEncoder) epsilonClosure(states map[int]bool) {
	queue := []int{}
	for s := range states {
		queue = append(queue, s)
	}

	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]

		// Get epsilon transitions (ch = 0)
		if epsTargets, ok := d.nfa[state].Trans[0]; ok {
			for target := range epsTargets {
				if !states[target] {
					states[target] = true
					queue = append(queue, target)
				}
			}
		}
	}
}

// buildDFA performs subset construction
func (d *DankEncoder) buildDFA() {
	d.dfaStates = make(map[string]int)
	startKey := d.setKey(d.initStates)
	d.dfaStates[startKey] = 0

	initList := []int{}
	for s := range d.initStates {
		initList = append(initList, s)
	}
	sort.Ints(initList)

	d.dfa = []*DFAState{
		{
			ID:      0,
			NFAIDs:  initList,
			Trans:   make(map[byte]int),
			IsFinal: d.hasAcceptingState(d.initStates),
		},
	}

	queue := []int{0}

	for len(queue) > 0 {
		currID := queue[0]
		queue = queue[1:]
		curr := d.dfa[currID]

		currStates := make(map[int]bool)
		for _, nid := range curr.NFAIDs {
			currStates[nid] = true
		}

		// Group transitions by character (skip epsilon = 0)
		charMap := make(map[byte]map[int]bool)
		for nfaID := range currStates {
			for ch, targets := range d.nfa[nfaID].Trans {
				if ch == 0 {
					continue // Skip epsilon
				}
				if charMap[ch] == nil {
					charMap[ch] = make(map[int]bool)
				}
				for target := range targets {
					charMap[ch][target] = true
				}
			}
		}

		// Process each character
		for ch, moveSet := range charMap {
			// Compute epsilon closure
			d.epsilonClosure(moveSet)

			key := d.setKey(moveSet)
			nextID, exists := d.dfaStates[key]

			if !exists {
				nextID = len(d.dfa)
				d.dfaStates[key] = nextID

				moveList := []int{}
				for s := range moveSet {
					moveList = append(moveList, s)
				}
				sort.Ints(moveList)

				newState := &DFAState{
					ID:      nextID,
					NFAIDs:  moveList,
					Trans:   make(map[byte]int),
					IsFinal: d.hasAcceptingState(moveSet),
				}
				d.dfa = append(d.dfa, newState)
				queue = append(queue, nextID)
			}

			curr.Trans[ch] = nextID
		}
	}

	// Add dead state (like Python's C++ implementation)
	// The dead state is a non-final state with all transitions to itself
	deadStateID := len(d.dfa)
	deadState := &DFAState{
		ID:      deadStateID,
		NFAIDs:  []int{}, // No NFA states
		Trans:   make(map[byte]int),
		IsFinal: false,
	}
	// All missing transitions in other states should point to dead state
	// And dead state transitions to itself
	for _, ch := range d.alphabet {
		deadState.Trans[ch] = deadStateID
	}
	d.dfa = append(d.dfa, deadState)

	// Update all states to have complete transition functions pointing to dead state
	for _, state := range d.dfa[:deadStateID] { // Don't process dead state itself
		for _, ch := range d.alphabet {
			if _, exists := state.Trans[ch]; !exists {
				state.Trans[ch] = deadStateID
			}
		}
	}
}

// hasAcceptingState checks if any NFA state in set is final
func (d *DankEncoder) hasAcceptingState(states map[int]bool) bool {
	for s := range states {
		if s < len(d.nfa) && d.nfa[s].IsFinal {
			return true
		}
	}
	return false
}

// setKey creates a unique key for a set of states
func (d *DankEncoder) setKey(states map[int]bool) string {
	ids := []int{}
	for s := range states {
		ids = append(ids, s)
	}
	sort.Ints(ids)

	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = strconv.Itoa(id)
	}
	return strings.Join(strs, ",")
}

// NumWords counts accepted strings using DP
func (d *DankEncoder) NumWords(minLen, maxLen int) int64 {
	if maxLen > d.fixedSlice {
		maxLen = d.fixedSlice
	}

	var total big.Int
	dp := make([]map[int]*big.Int, maxLen+1)
	for i := range dp {
		dp[i] = make(map[int]*big.Int)
	}
	dp[0][0] = big.NewInt(1)

	// Don't count dead state in DP
	deadState := len(d.dfa) - 1

	for l := 1; l <= maxLen; l++ {
		for state, ways := range dp[l-1] {
			// Iterate over actual transitions, not just alphabet
			// (pattern may contain characters outside alphabet like *)
			for _, next := range d.dfa[state].Trans {
				// Skip transitions to dead state
				if next == deadState {
					continue
				}
				if _, has := dp[l][next]; !has {
					dp[l][next] = big.NewInt(0)
				}
				dp[l][next].Add(dp[l][next], ways)
			}
		}
	}

	for l := minLen; l <= maxLen; l++ {
		for state, ways := range dp[l] {
			if d.dfa[state].IsFinal {
				total.Add(&total, ways)
			}
		}
	}

	return total.Int64()
}

// GenerateAtFixedLength returns all strings of exactly fixedLen
func (d *DankEncoder) GenerateAtFixedLength(fixedLen int) []string {
	var results []string
	d.dfsGenerateFixed(0, "", fixedLen, &results)
	sort.Strings(results)
	return results
}

// dfsGenerateFixed generates only strings of exact length
func (d *DankEncoder) dfsGenerateFixed(state int, curr string, remaining int, results *[]string) {
	// Skip dead state (last state in DFA)
	deadState := len(d.dfa) - 1
	if state == deadState {
		return
	}

	if remaining == 0 {
		if d.dfa[state].IsFinal {
			*results = append(*results, curr)
		}
		return
	}

	// Iterate over actual transitions (sorted for deterministic output)
	// Can't just use alphabet because pattern may have characters outside alphabet (like *)
	chars := []byte{}
	for ch := range d.dfa[state].Trans {
		chars = append(chars, ch)
	}
	sort.Slice(chars, func(i, j int) bool { return chars[i] < chars[j] })

	for _, ch := range chars {
		next := d.dfa[state].Trans[ch]
		// Don't transition to dead state during generation
		if next != deadState {
			d.dfsGenerateFixed(next, curr+string(ch), remaining-1, results)
		}
	}
}

// NumStates returns the number of DFA states
func (d *DankEncoder) NumStates() int {
	return len(d.dfa)
}

// NumNFAStates returns the number of NFA states (for debugging)
func (d *DankEncoder) NumNFAStates() int {
	return len(d.nfa)
}

// reverseDFA converts the current DFA back to an NFA with reversed transitions
// This is part of Brzozowski's algorithm for DFA minimization
func (d *DankEncoder) reverseDFA() {
	// Create new NFA with same number of states as DFA
	newNFA := make([]*NFAState, len(d.dfa))
	for i := range newNFA {
		newNFA[i] = &NFAState{
			ID:      i,
			Trans:   make(map[byte]map[int]bool),
			IsFinal: false,
		}
	}

	// The old init state becomes the only final state
	newNFA[0].IsFinal = true

	// Reverse all transitions
	for _, state := range d.dfa {
		for ch, target := range state.Trans {
			// Add reverse transition: target --ch--> state.ID
			if newNFA[target].Trans[ch] == nil {
				newNFA[target].Trans[ch] = make(map[int]bool)
			}
			newNFA[target].Trans[ch][state.ID] = true
		}
	}

	// Old final states become new init states
	newInitStates := make(map[int]bool)
	for _, state := range d.dfa {
		if state.IsFinal {
			newInitStates[state.ID] = true
		}
	}

	// Replace NFA
	d.nfa = newNFA
	d.initStates = newInitStates
	d.stateCounter = len(newNFA)

	// Compute epsilon closure of init states (no epsilon transitions after reverse, but for consistency)
	d.epsilonClosure(d.initStates)
}
