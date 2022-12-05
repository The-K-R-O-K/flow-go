// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.9
// source: macro_benchmark.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type StartMacroBenchmarkRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *StartMacroBenchmarkRequest) Reset() {
	*x = StartMacroBenchmarkRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_macro_benchmark_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StartMacroBenchmarkRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StartMacroBenchmarkRequest) ProtoMessage() {}

func (x *StartMacroBenchmarkRequest) ProtoReflect() protoreflect.Message {
	mi := &file_macro_benchmark_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StartMacroBenchmarkRequest.ProtoReflect.Descriptor instead.
func (*StartMacroBenchmarkRequest) Descriptor() ([]byte, []int) {
	return file_macro_benchmark_proto_rawDescGZIP(), []int{0}
}

type StartMacroBenchmarkResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *StartMacroBenchmarkResponse) Reset() {
	*x = StartMacroBenchmarkResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_macro_benchmark_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StartMacroBenchmarkResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StartMacroBenchmarkResponse) ProtoMessage() {}

func (x *StartMacroBenchmarkResponse) ProtoReflect() protoreflect.Message {
	mi := &file_macro_benchmark_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StartMacroBenchmarkResponse.ProtoReflect.Descriptor instead.
func (*StartMacroBenchmarkResponse) Descriptor() ([]byte, []int) {
	return file_macro_benchmark_proto_rawDescGZIP(), []int{1}
}

type GetMacroBenchmarkRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetMacroBenchmarkRequest) Reset() {
	*x = GetMacroBenchmarkRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_macro_benchmark_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetMacroBenchmarkRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetMacroBenchmarkRequest) ProtoMessage() {}

func (x *GetMacroBenchmarkRequest) ProtoReflect() protoreflect.Message {
	mi := &file_macro_benchmark_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetMacroBenchmarkRequest.ProtoReflect.Descriptor instead.
func (*GetMacroBenchmarkRequest) Descriptor() ([]byte, []int) {
	return file_macro_benchmark_proto_rawDescGZIP(), []int{2}
}

type GetMacroBenchmarkResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetMacroBenchmarkResponse) Reset() {
	*x = GetMacroBenchmarkResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_macro_benchmark_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetMacroBenchmarkResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetMacroBenchmarkResponse) ProtoMessage() {}

func (x *GetMacroBenchmarkResponse) ProtoReflect() protoreflect.Message {
	mi := &file_macro_benchmark_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetMacroBenchmarkResponse.ProtoReflect.Descriptor instead.
func (*GetMacroBenchmarkResponse) Descriptor() ([]byte, []int) {
	return file_macro_benchmark_proto_rawDescGZIP(), []int{3}
}

type ListMacroBenchmarksResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ListMacroBenchmarksResponse) Reset() {
	*x = ListMacroBenchmarksResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_macro_benchmark_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListMacroBenchmarksResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListMacroBenchmarksResponse) ProtoMessage() {}

func (x *ListMacroBenchmarksResponse) ProtoReflect() protoreflect.Message {
	mi := &file_macro_benchmark_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListMacroBenchmarksResponse.ProtoReflect.Descriptor instead.
func (*ListMacroBenchmarksResponse) Descriptor() ([]byte, []int) {
	return file_macro_benchmark_proto_rawDescGZIP(), []int{4}
}

type StatusResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *StatusResponse) Reset() {
	*x = StatusResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_macro_benchmark_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StatusResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StatusResponse) ProtoMessage() {}

func (x *StatusResponse) ProtoReflect() protoreflect.Message {
	mi := &file_macro_benchmark_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StatusResponse.ProtoReflect.Descriptor instead.
func (*StatusResponse) Descriptor() ([]byte, []int) {
	return file_macro_benchmark_proto_rawDescGZIP(), []int{5}
}

var File_macro_benchmark_proto protoreflect.FileDescriptor

var file_macro_benchmark_proto_rawDesc = []byte{
	0x0a, 0x15, 0x6d, 0x61, 0x63, 0x72, 0x6f, 0x5f, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72,
	0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x09, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61,
	0x72, 0x6b, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0x1c, 0x0a, 0x1a, 0x53, 0x74, 0x61, 0x72, 0x74, 0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e,
	0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x1d, 0x0a,
	0x1b, 0x53, 0x74, 0x61, 0x72, 0x74, 0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68,
	0x6d, 0x61, 0x72, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x1a, 0x0a, 0x18,
	0x47, 0x65, 0x74, 0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72,
	0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x1b, 0x0a, 0x19, 0x47, 0x65, 0x74, 0x4d,
	0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x1d, 0x0a, 0x1b, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x61, 0x63,
	0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x73, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x22, 0x10, 0x0a, 0x0e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0xef, 0x02, 0x0a, 0x09, 0x42, 0x65, 0x6e, 0x63, 0x68,
	0x6d, 0x61, 0x72, 0x6b, 0x12, 0x68, 0x0a, 0x13, 0x53, 0x74, 0x61, 0x72, 0x74, 0x4d, 0x61, 0x63,
	0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x12, 0x25, 0x2e, 0x62, 0x65,
	0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x2e, 0x53, 0x74, 0x61, 0x72, 0x74, 0x4d, 0x61, 0x63,
	0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x26, 0x2e, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x2e, 0x53,
	0x74, 0x61, 0x72, 0x74, 0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61,
	0x72, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x30, 0x01, 0x12, 0x60,
	0x0a, 0x11, 0x47, 0x65, 0x74, 0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d,
	0x61, 0x72, 0x6b, 0x12, 0x23, 0x2e, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x2e,
	0x47, 0x65, 0x74, 0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72,
	0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x24, 0x2e, 0x62, 0x65, 0x6e, 0x63, 0x68,
	0x6d, 0x61, 0x72, 0x6b, 0x2e, 0x47, 0x65, 0x74, 0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e,
	0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x12, 0x57, 0x0a, 0x13, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e,
	0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x73, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a,
	0x26, 0x2e, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x2e, 0x4c, 0x69, 0x73, 0x74,
	0x4d, 0x61, 0x63, 0x72, 0x6f, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x73, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3d, 0x0a, 0x06, 0x53, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x19, 0x2e, 0x62, 0x65,
	0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x37, 0x5a, 0x35, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6f, 0x6e, 0x66, 0x6c, 0x6f, 0x77, 0x2f, 0x66, 0x6c,
	0x6f, 0x77, 0x2d, 0x67, 0x6f, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2f, 0x62, 0x65, 0x63, 0x6e, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_macro_benchmark_proto_rawDescOnce sync.Once
	file_macro_benchmark_proto_rawDescData = file_macro_benchmark_proto_rawDesc
)

func file_macro_benchmark_proto_rawDescGZIP() []byte {
	file_macro_benchmark_proto_rawDescOnce.Do(func() {
		file_macro_benchmark_proto_rawDescData = protoimpl.X.CompressGZIP(file_macro_benchmark_proto_rawDescData)
	})
	return file_macro_benchmark_proto_rawDescData
}

var file_macro_benchmark_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_macro_benchmark_proto_goTypes = []interface{}{
	(*StartMacroBenchmarkRequest)(nil),  // 0: benchmark.StartMacroBenchmarkRequest
	(*StartMacroBenchmarkResponse)(nil), // 1: benchmark.StartMacroBenchmarkResponse
	(*GetMacroBenchmarkRequest)(nil),    // 2: benchmark.GetMacroBenchmarkRequest
	(*GetMacroBenchmarkResponse)(nil),   // 3: benchmark.GetMacroBenchmarkResponse
	(*ListMacroBenchmarksResponse)(nil), // 4: benchmark.ListMacroBenchmarksResponse
	(*StatusResponse)(nil),              // 5: benchmark.StatusResponse
	(*emptypb.Empty)(nil),               // 6: google.protobuf.Empty
}
var file_macro_benchmark_proto_depIdxs = []int32{
	0, // 0: benchmark.Benchmark.StartMacroBenchmark:input_type -> benchmark.StartMacroBenchmarkRequest
	2, // 1: benchmark.Benchmark.GetMacroBenchmark:input_type -> benchmark.GetMacroBenchmarkRequest
	6, // 2: benchmark.Benchmark.ListMacroBenchmarks:input_type -> google.protobuf.Empty
	6, // 3: benchmark.Benchmark.Status:input_type -> google.protobuf.Empty
	1, // 4: benchmark.Benchmark.StartMacroBenchmark:output_type -> benchmark.StartMacroBenchmarkResponse
	3, // 5: benchmark.Benchmark.GetMacroBenchmark:output_type -> benchmark.GetMacroBenchmarkResponse
	4, // 6: benchmark.Benchmark.ListMacroBenchmarks:output_type -> benchmark.ListMacroBenchmarksResponse
	5, // 7: benchmark.Benchmark.Status:output_type -> benchmark.StatusResponse
	4, // [4:8] is the sub-list for method output_type
	0, // [0:4] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_macro_benchmark_proto_init() }
func file_macro_benchmark_proto_init() {
	if File_macro_benchmark_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_macro_benchmark_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StartMacroBenchmarkRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_macro_benchmark_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StartMacroBenchmarkResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_macro_benchmark_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetMacroBenchmarkRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_macro_benchmark_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetMacroBenchmarkResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_macro_benchmark_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListMacroBenchmarksResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_macro_benchmark_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StatusResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_macro_benchmark_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_macro_benchmark_proto_goTypes,
		DependencyIndexes: file_macro_benchmark_proto_depIdxs,
		MessageInfos:      file_macro_benchmark_proto_msgTypes,
	}.Build()
	File_macro_benchmark_proto = out.File
	file_macro_benchmark_proto_rawDesc = nil
	file_macro_benchmark_proto_goTypes = nil
	file_macro_benchmark_proto_depIdxs = nil
}