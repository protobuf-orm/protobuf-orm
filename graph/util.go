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

	if _, ok := p.(Field); !ok {
		return false
	}
	if p.Descriptor().IsMap() {
		return true
	}

	return false
}
