// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: orm/patch_op.proto

package ormpb

import (
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

type PatchOp int32

const (
	PatchOp_PATCH_OP_UNSPECIFIED PatchOp = 0
	PatchOp_PATCH_OP_ADD         PatchOp = 1
	PatchOp_PATCH_OP_ERASE       PatchOp = 2
	PatchOp_PATCH_OP_CLEAR       PatchOp = 3
)

// Enum value maps for PatchOp.
var (
	PatchOp_name = map[int32]string{
		0: "PATCH_OP_UNSPECIFIED",
		1: "PATCH_OP_ADD",
		2: "PATCH_OP_ERASE",
		3: "PATCH_OP_CLEAR",
	}
	PatchOp_value = map[string]int32{
		"PATCH_OP_UNSPECIFIED": 0,
		"PATCH_OP_ADD":         1,
		"PATCH_OP_ERASE":       2,
		"PATCH_OP_CLEAR":       3,
	}
)

func (x PatchOp) Enum() *PatchOp {
	p := new(PatchOp)
	*p = x
	return p
}

func (x PatchOp) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PatchOp) Descriptor() protoreflect.EnumDescriptor {
	return file_orm_patch_op_proto_enumTypes[0].Descriptor()
}

func (PatchOp) Type() protoreflect.EnumType {
	return &file_orm_patch_op_proto_enumTypes[0]
}

func (x PatchOp) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

var File_orm_patch_op_proto protoreflect.FileDescriptor

const file_orm_patch_op_proto_rawDesc = "" +
	"\n" +
	"\x12orm/patch_op.proto\x12\x03orm*]\n" +
	"\aPatchOp\x12\x18\n" +
	"\x14PATCH_OP_UNSPECIFIED\x10\x00\x12\x10\n" +
	"\fPATCH_OP_ADD\x10\x01\x12\x12\n" +
	"\x0ePATCH_OP_ERASE\x10\x02\x12\x12\n" +
	"\x0ePATCH_OP_CLEAR\x10\x03B,Z*github.com/protobuf-orm/protobuf-orm/ormpbb\beditionsp\xe8\a"

var file_orm_patch_op_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_orm_patch_op_proto_goTypes = []any{
	(PatchOp)(0), // 0: orm.PatchOp
}
var file_orm_patch_op_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_orm_patch_op_proto_init() }
func file_orm_patch_op_proto_init() {
	if File_orm_patch_op_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_orm_patch_op_proto_rawDesc), len(file_orm_patch_op_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_orm_patch_op_proto_goTypes,
		DependencyIndexes: file_orm_patch_op_proto_depIdxs,
		EnumInfos:         file_orm_patch_op_proto_enumTypes,
	}.Build()
	File_orm_patch_op_proto = out.File
	file_orm_patch_op_proto_goTypes = nil
	file_orm_patch_op_proto_depIdxs = nil
}
