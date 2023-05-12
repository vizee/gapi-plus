package descriptor

import (
	annotation "github.com/vizee/gapi-proto-go/gapi"
	"google.golang.org/protobuf/types/descriptorpb"
)

type FieldDesc struct {
	Name      string
	Type      descriptorpb.FieldDescriptorProto_Type
	Ref       *MessageDesc
	Tag       int32
	Label     descriptorpb.FieldDescriptorProto_Label
	Alias     string
	Bind      annotation.FIELD_BIND
	OmitEmpty bool
}

type MessageDesc struct {
	Name       string
	Fields     []FieldDesc
	MapEntry   bool
	Incomplete bool
}

type ServiceOptions struct {
	Server         string
	DefaultHandler string
	DefaultTimeout int64
	PathPrefix     string
	Use            []string
}

type ServiceDesc struct {
	Name     string
	FullName string
	Methods  []*MethodDesc
	Opts     ServiceOptions
}

type MethodDesc struct {
	Name      string
	In        *MessageDesc
	Out       *MessageDesc
	Streaming bool
	Opts      MethodOptions
}

type MethodOptions struct {
	Method  string
	Path    string
	Use     []string
	Timeout int64
	Handler string
}
