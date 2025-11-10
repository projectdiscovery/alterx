package patternmining

import "sort"

// editClosures computes edit-distance closures for clustering
func (m *Miner) editClosures(items []string, delta int) [][]string {
	var ret [][]string
	for _, a := range items {
		rSet := make(map[string]bool)
		rSet[a] = true
		for _, b := range items {
			d := m.getDist(a, b)
			if d < delta {
				rSet[b] = true
			}
		}
		r := []string{}
		for k := range rSet {
			r = append(r, k)
		}
		found := false
		for _, s := range ret {
			if sameSlices(r, s) {
				found = true
				break
			}
		}
		if !found {
			ret = append(ret, r)
		}
	}
	return ret
}

func sameSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := make([]string, len(a))
	copy(sa, a)
	sb := make([]string, len(b))
	copy(sb, b)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

