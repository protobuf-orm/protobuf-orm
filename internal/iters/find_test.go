package iters_test

import (
	"slices"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/internal/iters"
	"github.com/stretchr/testify/require"
)

func TestFind(t *testing.T) {
	seq := slices.Values([]int{2, 4, 6, 8})
	t.Run("found", func(t *testing.T) {
		x := require.New(t)
		v, ok := iters.Find(seq, func(v int) bool { return v == 6 })
		x.True(ok)
		x.Equal(6, v)
	})
	t.Run("first match wins", func(t *testing.T) {
		x := require.New(t)
		v, ok := iters.Find(seq, func(v int) bool { return v%4 == 0 })
		x.True(ok)
		x.Equal(4, v)
	})
	t.Run("not found returns zero value", func(t *testing.T) {
		x := require.New(t)
		v, ok := iters.Find(seq, func(v int) bool { return v == 5 })
		x.False(ok)
		x.Equal(0, v)
	})
	t.Run("empty sequence", func(t *testing.T) {
		x := require.New(t)
		v, ok := iters.Find(slices.Values([]int{}), func(int) bool { return true })
		x.False(ok)
		x.Equal(0, v)
	})
}
