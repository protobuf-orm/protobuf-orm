package ormpb_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestTypeFromKind(t *testing.T) {
	for _, tc := range []struct {
		kind protoreflect.Kind
		want ormpb.Type
	}{
		{protoreflect.BoolKind, ormpb.Type_TYPE_BOOL},
		{protoreflect.EnumKind, ormpb.Type_TYPE_ENUM},
		{protoreflect.Int32Kind, ormpb.Type_TYPE_INT32},
		{protoreflect.Sint32Kind, ormpb.Type_TYPE_SINT32},
		{protoreflect.Uint32Kind, ormpb.Type_TYPE_UINT32},
		{protoreflect.Int64Kind, ormpb.Type_TYPE_INT64},
		{protoreflect.Sint64Kind, ormpb.Type_TYPE_SINT64},
		{protoreflect.Uint64Kind, ormpb.Type_TYPE_UINT64},
		{protoreflect.Sfixed32Kind, ormpb.Type_TYPE_SFIXED32},
		{protoreflect.Fixed32Kind, ormpb.Type_TYPE_FIXED32},
		{protoreflect.FloatKind, ormpb.Type_TYPE_FLOAT},
		{protoreflect.Sfixed64Kind, ormpb.Type_TYPE_SFIXED64},
		{protoreflect.Fixed64Kind, ormpb.Type_TYPE_FIXED64},
		{protoreflect.DoubleKind, ormpb.Type_TYPE_DOUBLE},
		{protoreflect.StringKind, ormpb.Type_TYPE_STRING},
		{protoreflect.BytesKind, ormpb.Type_TYPE_BYTES},
		{protoreflect.MessageKind, ormpb.Type_TYPE_MESSAGE},
		{protoreflect.GroupKind, ormpb.Type_TYPE_GROUP},
	} {
		t.Run(tc.kind.String(), func(t *testing.T) {
			require.Equal(t, tc.want, ormpb.TypeFromKind(tc.kind))
		})
	}
}

func TestDeduceType(t *testing.T) {
	md := graphtest.File_graphtest_field_type_mapping_proto.Messages().ByName("FieldTypeMapping")
	require.NotNil(t, md)
	by := func(name string) protoreflect.FieldDescriptor {
		f := md.Fields().ByName(protoreflect.Name(name))
		require.NotNilf(t, f, "field %q not found", name)
		return f
	}
	for _, tc := range []struct {
		field string
		want  ormpb.Type
	}{
		{"v_i32", ormpb.Type_TYPE_INT32},
		{"v_string", ormpb.Type_TYPE_STRING},
		{"v_bytes", ormpb.Type_TYPE_BYTES},
		{"v_bool", ormpb.Type_TYPE_BOOL},
		{"wkt_time", ormpb.Type_TYPE_TIME},
		{"wkt_struct", ormpb.Type_TYPE_JSON},
		{"wkt_value", ormpb.Type_TYPE_JSON},
		{"v_message", ormpb.Type_TYPE_JSON},
	} {
		t.Run(tc.field, func(t *testing.T) {
			require.Equal(t, tc.want, ormpb.DeduceType(by(tc.field)))
		})
	}
}

func TestDeduceTypeMap(t *testing.T) {
	md := graphtest.File_graphtest_field_proto.Messages().ByName("MapField")
	require.NotNil(t, md)
	f := md.Fields().ByName("implicit_string")
	require.NotNil(t, f)
	require.Equal(t, ormpb.Type_TYPE_JSON, ormpb.DeduceType(f))
}

func TestTypeDecay(t *testing.T) {
	for _, tc := range []struct {
		in   ormpb.Type
		want ormpb.Type
	}{
		{ormpb.Type_TYPE_FLOAT, ormpb.Type_TYPE_FLOAT},
		{ormpb.Type_TYPE_DOUBLE, ormpb.Type_TYPE_FLOAT},
		{ormpb.Type_TYPE_INT32, ormpb.Type_TYPE_INT},
		{ormpb.Type_TYPE_SFIXED64, ormpb.Type_TYPE_INT},
		{ormpb.Type_TYPE_UINT64, ormpb.Type_TYPE_UINT},
		{ormpb.Type_TYPE_FIXED32, ormpb.Type_TYPE_UINT},
		// An enum is integer-backed, so it decays to INT (not MESSAGE).
		{ormpb.Type_TYPE_ENUM, ormpb.Type_TYPE_INT},
		{ormpb.Type_TYPE_TIME, ormpb.Type_TYPE_MESSAGE},
		{ormpb.Type_TYPE_JSON, ormpb.Type_TYPE_MESSAGE},
		{ormpb.Type_TYPE_MESSAGE, ormpb.Type_TYPE_MESSAGE},
		{ormpb.Type_TYPE_GROUP, ormpb.Type_TYPE_MESSAGE},
		{ormpb.Type_TYPE_UUID, ormpb.Type_TYPE_BYTES},
		{ormpb.Type_TYPE_BOOL, ormpb.Type_TYPE_BOOL},
		{ormpb.Type_TYPE_STRING, ormpb.Type_TYPE_STRING},
	} {
		t.Run(tc.in.String(), func(t *testing.T) {
			require.Equal(t, tc.want, tc.in.Decay())
		})
	}
}

func TestTypeIsScalarIsMessage(t *testing.T) {
	x := require.New(t)
	x.True(ormpb.Type_TYPE_STRING.IsScalar())
	x.True(ormpb.Type_TYPE_INT32.IsScalar())
	x.True(ormpb.Type_TYPE_UUID.IsScalar()) // UUID decays to bytes.
	x.True(ormpb.Type_TYPE_ENUM.IsScalar()) // enum decays to int.
	x.False(ormpb.Type_TYPE_STRING.IsMessage())
	x.False(ormpb.Type_TYPE_ENUM.IsMessage())
	x.True(ormpb.Type_TYPE_TIME.IsMessage())
	x.True(ormpb.Type_TYPE_JSON.IsMessage())
	x.True(ormpb.Type_TYPE_MESSAGE.IsMessage())
}
