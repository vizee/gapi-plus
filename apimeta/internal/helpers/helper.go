package helpers

import (
	"strings"

	"github.com/vizee/jsonpb"
	"google.golang.org/protobuf/types/descriptorpb"
)

func ConcatFullMethodName(serviceName string, methodName string) string {
	var s strings.Builder
	s.Grow(2 + len(serviceName) + len(methodName))
	s.WriteByte('/')
	s.WriteString(serviceName)
	s.WriteByte('/')
	s.WriteString(methodName)
	return s.String()
}

func CheckMiddlewareName(name string) bool {
	if name == "" {
		return false
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		if 'a' <= c && c <= 'z' ||
			'A' <= c && c <= 'Z' ||
			'0' <= c && c <= '9' ||
			c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func GetTypeKind(ty descriptorpb.FieldDescriptorProto_Type) (jsonpb.Kind, bool) {
	switch ty {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return jsonpb.DoubleKind, true
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return jsonpb.FloatKind, true
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return jsonpb.Int64Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return jsonpb.Uint64Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return jsonpb.Int32Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return jsonpb.Fixed64Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return jsonpb.Fixed32Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return jsonpb.BoolKind, true
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return jsonpb.StringKind, true
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return jsonpb.MessageKind, true
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return jsonpb.BytesKind, true
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return jsonpb.Uint32Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return jsonpb.Int32Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return jsonpb.Sfixed32Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return jsonpb.Sfixed64Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		return jsonpb.Sint32Kind, true
	case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return jsonpb.Sint64Kind, true
	}
	return 0, false
}
