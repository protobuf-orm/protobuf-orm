// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: graphtest/o2m_parent_marked_unique.proto

package graphtest

import (
	_ "github.com/protobuf-orm/protobuf-orm/ormpb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type O2MParentMarkedUnique struct {
	state                  protoimpl.MessageState    `protogen:"opaque.v1"`
	xxx_hidden_Id          int32                     `protobuf:"varint,1,opt,name=id"`
	xxx_hidden_Parent      *O2MParentMarkedUnique    `protobuf:"bytes,10,opt,name=parent"`
	xxx_hidden_Children    *[]*O2MParentMarkedUnique `protobuf:"bytes,11,rep,name=children"`
	XXX_raceDetectHookData protoimpl.RaceDetectHookData
	XXX_presence           [1]uint32
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *O2MParentMarkedUnique) Reset() {
	*x = O2MParentMarkedUnique{}
	mi := &file_graphtest_o2m_parent_marked_unique_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *O2MParentMarkedUnique) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*O2MParentMarkedUnique) ProtoMessage() {}

func (x *O2MParentMarkedUnique) ProtoReflect() protoreflect.Message {
	mi := &file_graphtest_o2m_parent_marked_unique_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *O2MParentMarkedUnique) GetId() int32 {
	if x != nil {
		return x.xxx_hidden_Id
	}
	return 0
}

func (x *O2MParentMarkedUnique) GetParent() *O2MParentMarkedUnique {
	if x != nil {
		return x.xxx_hidden_Parent
	}
	return nil
}

func (x *O2MParentMarkedUnique) GetChildren() []*O2MParentMarkedUnique {
	if x != nil {
		if x.xxx_hidden_Children != nil {
			return *x.xxx_hidden_Children
		}
	}
	return nil
}

func (x *O2MParentMarkedUnique) SetId(v int32) {
	x.xxx_hidden_Id = v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 0, 3)
}

func (x *O2MParentMarkedUnique) SetParent(v *O2MParentMarkedUnique) {
	x.xxx_hidden_Parent = v
}

func (x *O2MParentMarkedUnique) SetChildren(v []*O2MParentMarkedUnique) {
	x.xxx_hidden_Children = &v
}

func (x *O2MParentMarkedUnique) HasId() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 0)
}

func (x *O2MParentMarkedUnique) HasParent() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_Parent != nil
}

func (x *O2MParentMarkedUnique) ClearId() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 0)
	x.xxx_hidden_Id = 0
}

func (x *O2MParentMarkedUnique) ClearParent() {
	x.xxx_hidden_Parent = nil
}

type O2MParentMarkedUnique_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Id       *int32
	Parent   *O2MParentMarkedUnique
	Children []*O2MParentMarkedUnique
}

func (b0 O2MParentMarkedUnique_builder) Build() *O2MParentMarkedUnique {
	m0 := &O2MParentMarkedUnique{}
	b, x := &b0, m0
	_, _ = b, x
	if b.Id != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 0, 3)
		x.xxx_hidden_Id = *b.Id
	}
	x.xxx_hidden_Parent = b.Parent
	x.xxx_hidden_Children = &b.Children
	return m0
}

var File_graphtest_o2m_parent_marked_unique_proto protoreflect.FileDescriptor

const file_graphtest_o2m_parent_marked_unique_proto_rawDesc = "" +
	"\n" +
	"(graphtest/o2m_parent_marked_unique.proto\x12\tgraphtest\x1a\torm.proto\"\xc7\x01\n" +
	"\x15O2mParentMarkedUnique\x12\x16\n" +
	"\x02id\x18\x01 \x01(\x05B\x06\xea\x82\x16\x02(\x01R\x02id\x12@\n" +
	"\x06parent\x18\n" +
	" \x01(\v2 .graphtest.O2mParentMarkedUniqueB\x06\xf2\x82\x16\x020\x01R\x06parent\x12N\n" +
	"\bchildren\x18\v \x03(\v2 .graphtest.O2mParentMarkedUniqueB\x10\xf2\x82\x16\f\x1a\n" +
	"\n" +
	"\x06parent\x10\n" +
	"R\bchildren:\x04\xca\xfc\x15\x00BBZ@github.com/protobuf-orm/protobuf-orm/internal/examples/graphtestb\beditionsp\xe8\a"

var file_graphtest_o2m_parent_marked_unique_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_graphtest_o2m_parent_marked_unique_proto_goTypes = []any{
	(*O2MParentMarkedUnique)(nil), // 0: graphtest.O2mParentMarkedUnique
}
var file_graphtest_o2m_parent_marked_unique_proto_depIdxs = []int32{
	0, // 0: graphtest.O2mParentMarkedUnique.parent:type_name -> graphtest.O2mParentMarkedUnique
	0, // 1: graphtest.O2mParentMarkedUnique.children:type_name -> graphtest.O2mParentMarkedUnique
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_graphtest_o2m_parent_marked_unique_proto_init() }
func file_graphtest_o2m_parent_marked_unique_proto_init() {
	if File_graphtest_o2m_parent_marked_unique_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_graphtest_o2m_parent_marked_unique_proto_rawDesc), len(file_graphtest_o2m_parent_marked_unique_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_graphtest_o2m_parent_marked_unique_proto_goTypes,
		DependencyIndexes: file_graphtest_o2m_parent_marked_unique_proto_depIdxs,
		MessageInfos:      file_graphtest_o2m_parent_marked_unique_proto_msgTypes,
	}.Build()
	File_graphtest_o2m_parent_marked_unique_proto = out.File
	file_graphtest_o2m_parent_marked_unique_proto_goTypes = nil
	file_graphtest_o2m_parent_marked_unique_proto_depIdxs = nil
}
