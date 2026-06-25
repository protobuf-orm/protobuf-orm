package ormpb_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// fields returns the field descriptors of library.User, which has well-known
// numbers/names (id=1, alias=4, name=5).
func userFields(t *testing.T) protoreflect.FieldDescriptors {
	t.Helper()
	md := library.File_library_user_proto.Messages().ByName("User")
	require.NotNil(t, md)
	return md.Fields()
}

func TestRefBuilders(t *testing.T) {
	t.Run("by number", func(t *testing.T) {
		r := ormpb.RefByNumber(5)
		require.EqualValues(t, 5, r.GetNumber())
		require.Equal(t, "", r.GetName())
	})
	t.Run("by name", func(t *testing.T) {
		r := ormpb.RefByName("name")
		require.Equal(t, "name", r.GetName())
		require.EqualValues(t, 0, r.GetNumber())
	})
}

func TestRefAccess(t *testing.T) {
	fields := userFields(t)
	t.Run("by number only", func(t *testing.T) {
		x := require.New(t)
		f, err := ormpb.RefByNumber(5).Access(fields)
		x.NoError(err)
		x.Equal("name", string(f.Name()))
	})
	t.Run("by name only", func(t *testing.T) {
		x := require.New(t)
		f, err := ormpb.RefByName("name").Access(fields)
		x.NoError(err)
		x.EqualValues(5, f.Number())
	})
	t.Run("by name and number", func(t *testing.T) {
		x := require.New(t)
		f, err := ormpb.Ref_builder{Name: "name", Number: 5}.Build().Access(fields)
		x.NoError(err)
		x.Equal("name", string(f.Name()))
		x.EqualValues(5, f.Number())
	})
	t.Run("empty ref", func(t *testing.T) {
		x := require.New(t)
		_, err := ormpb.Ref_builder{}.Build().Access(fields)
		x.ErrorContains(err, "empty ref")
	})
	t.Run("unknown number", func(t *testing.T) {
		x := require.New(t)
		_, err := ormpb.RefByNumber(9999).Access(fields)
		x.ErrorContains(err, "unknown number")
	})
	t.Run("unknown name", func(t *testing.T) {
		x := require.New(t)
		_, err := ormpb.RefByName("nope").Access(fields)
		x.ErrorContains(err, "unknown name")
	})
	t.Run("name and number mismatch", func(t *testing.T) {
		x := require.New(t)
		_, err := ormpb.Ref_builder{Name: "alias", Number: 5}.Build().Access(fields)
		x.ErrorContains(err, "name not matched")
	})
}
