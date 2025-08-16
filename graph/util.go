package graph

// Proto type of [p] has repeated cardinality.
// Means, it is defined like:
//
//	repeated string a = 1;
//	map[string, string] b = 2;
func IsCollection(p Prop) bool {
	if p.IsList() {
		return true
	}

	f, ok := p.(Field)
	if !ok {
		return false
	}

	if _, ok := f.Shape().(MapShape); ok {
		return true
	}

	return false
}
