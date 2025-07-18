// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: graphtest/key_wo_unique.proto

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

type KeyWoUnique struct {
	state                  protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Id          int32                  `protobuf:"varint,1,opt,name=id"`
	XXX_raceDetectHookData protoimpl.RaceDetectHookData
	XXX_presence           [1]uint32
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *KeyWoUnique) Reset() {
	*x = KeyWoUnique{}
	mi := &file_graphtest_key_wo_unique_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *KeyWoUnique) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KeyWoUnique) ProtoMessage() {}

func (x *KeyWoUnique) ProtoReflect() protoreflect.Message {
	mi := &file_graphtest_key_wo_unique_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *KeyWoUnique) GetId() int32 {
	if x != nil {
		return x.xxx_hidden_Id
	}
	return 0
}

func (x *KeyWoUnique) SetId(v int32) {
	x.xxx_hidden_Id = v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 0, 1)
}

func (x *KeyWoUnique) HasId() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 0)
}

func (x *KeyWoUnique) ClearId() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 0)
	x.xxx_hidden_Id = 0
}

type KeyWoUnique_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	// Key implies a unique so even if no unique is specified,
	// key is evaluated as unique.
	Id *int32
}

func (b0 KeyWoUnique_builder) Build() *KeyWoUnique {
	m0 := &KeyWoUnique{}
	b, x := &b0, m0
	_, _ = b, x
	if b.Id != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 0, 1)
		x.xxx_hidden_Id = *b.Id
	}
	return m0
}

var File_graphtest_key_wo_unique_proto protoreflect.FileDescriptor

const file_graphtest_key_wo_unique_proto_rawDesc = "" +
	"\n" +
	"\x1dgraphtest/key_wo_unique.proto\x12\tgraphtest\x1a\torm.proto\"+\n" +
	"\vKeyWoUnique\x12\x16\n" +
	"\x02id\x18\x01 \x01(\x05B\x06\xea\x82\x16\x02(\x01R\x02id:\x04\xca\xfc\x15\x00BBZ@github.com/protobuf-orm/protobuf-orm/internal/examples/graphtestb\beditionsp\xe8\a"

var file_graphtest_key_wo_unique_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_graphtest_key_wo_unique_proto_goTypes = []any{
	(*KeyWoUnique)(nil), // 0: graphtest.KeyWoUnique
}
var file_graphtest_key_wo_unique_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_graphtest_key_wo_unique_proto_init() }
func file_graphtest_key_wo_unique_proto_init() {
	if File_graphtest_key_wo_unique_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_graphtest_key_wo_unique_proto_rawDesc), len(file_graphtest_key_wo_unique_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_graphtest_key_wo_unique_proto_goTypes,
		DependencyIndexes: file_graphtest_key_wo_unique_proto_depIdxs,
		MessageInfos:      file_graphtest_key_wo_unique_proto_msgTypes,
	}.Build()
	File_graphtest_key_wo_unique_proto = out.File
	file_graphtest_key_wo_unique_proto_goTypes = nil
	file_graphtest_key_wo_unique_proto_depIdxs = nil
}
