package graph

import "iter"

func find[T any](it iter.Seq[T], f func(v T) bool) (T, bool) {
	for v := range it {
		if f(v) {
			return v, true
		}
	}

	var z T
	return z, false
}
