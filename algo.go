package alterx

// Nth Order ClusterBomb with variable length array/values
func ClusterBomb(payloads *IndexMap, callback func(varMap map[string]interface{}), Vector []string) {
	// The Goal of implementation is to reduce number of arbitary values by constructing a vector

	// Algorithm
	// step 1) Initialize/Input a IndexMap(Here: payloads)
	// indexMap is nothing but a map with all of keys indexed in a different map

	// step 2) Vector is n length array such that n = len(payloads)
	// Each value in payloads(IndexMap) contains a array
	// ex: payloads["word"] = []string{"api","dev","cloud"}

	// step 3) Initial length of Vector is 0 . By using recursion
	// we construct a Vector with all possible values of payloads[N] where N = 0 < len(payloads)

	// step 4) At end of recursion len(Vector) == len(payloads).Cap() - 1
	// which translates that Vn = {r0,r1,...,rn} and only rn is missing
	// in this case/situation iterate over all possible values of rn i.e payload.GetNth(n)

	// Debug: Check if payloads is empty
	if payloads.Cap() == 0 {
		// No payloads to expand - this will cause pattern to be returned unexpanded
		return
	}

	if len(Vector) == payloads.Cap()-1 {
		// end of vector
		vectorMap := map[string]interface{}{}
		for k, v := range Vector {
			// construct a map[variable]=value with all available vectors
			vectorMap[payloads.KeyAtNth(k)] = v
		}
		// one element a.k.a last element is missing from ^ map
		index := len(Vector)
		for _, elem := range payloads.GetNth(index) {
			vectorMap[payloads.KeyAtNth(index)] = elem
			callback(vectorMap)
		}
		return
	}

	// step 5) if vector is not filled until payload.Cap()-1
	// iterate over rth variable payloads and execute them using recursion
	// if Vector is empty or at 1st index fix iterate over xth position
	index := len(Vector)
	for _, v := range payloads.GetNth(index) {
		var tmp []string
		if len(Vector) > 0 {
			tmp = append(tmp, Vector...)
		}
		tmp = append(tmp, v)
		ClusterBomb(payloads, callback, tmp) // Recursion
	}
}

type IndexMap struct {
	values  map[string][]string
	indexes map[int]string
}

func (o *IndexMap) GetNth(n int) []string {
	return o.values[o.indexes[n]]
}

func (o *IndexMap) Cap() int {
	return len(o.values)
}

// KeyAtNth returns key present at Nth position
func (o *IndexMap) KeyAtNth(n int) string {
	return o.indexes[n]
}

// NewIndexMap returns type such that elements of map can be retrieved by a fixed index
func NewIndexMap(values map[string][]string) *IndexMap {
	i := &IndexMap{
		values: values,
	}
	indexes := map[int]string{}
	counter := 0
	for k := range values {
		indexes[counter] = k
		counter++
	}
	i.indexes = indexes
	return i
}
