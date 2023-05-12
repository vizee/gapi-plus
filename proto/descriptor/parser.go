package descriptor

import (
	"strings"

	annotation "github.com/vizee/gapi-proto-go/gapi"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Parser struct {
	ns     []string
	prefix string
	msgs   map[string]*MessageDesc
	svcs   []*ServiceDesc
}

func NewParser() *Parser {
	return &Parser{
		msgs: make(map[string]*MessageDesc),
	}
}

func (p *Parser) enter(ns string) {
	p.ns = append(p.ns, ns)
	p.prefix = "." + strings.Join(p.ns, ".")
}

func (p *Parser) leave() {
	p.ns = p.ns[:len(p.ns)-1]
	p.prefix = "." + strings.Join(p.ns, ".")
}

func normalName(name string) string {
	if len(name) > 0 && name[0] == '.' {
		return name[1:]
	}
	return name
}

func (p *Parser) getMessage(fullName string) *MessageDesc {
	msg := p.msgs[fullName]
	if msg == nil {
		msg = &MessageDesc{
			Name:       normalName(fullName),
			Incomplete: true,
		}
		p.msgs[fullName] = msg
	}
	return msg
}

func (p *Parser) parseMessage(md *descriptorpb.DescriptorProto) {
	p.enter(md.GetName())
	defer p.leave()

	fullName := p.prefix
	msg := p.getMessage(fullName)
	if !msg.Incomplete {
		panic(`Message '` + fullName + `' has already been parsed.`)
	}

	for _, nested := range md.NestedType {
		p.parseMessage(nested)
	}

	fields := make([]FieldDesc, 0, len(md.Field))
	for _, fd := range md.Field {
		ty := fd.GetType()
		var msgRef *MessageDesc
		if ty == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			refName := fd.GetTypeName()
			if !strings.HasPrefix(refName, ".") {
				// 这里只搜索了嵌套 scope，按照标准实现应该向上搜索到根
				refName = p.prefix + "." + refName
			}
			msgRef = p.getMessage(refName)
		}

		fields = append(fields, FieldDesc{
			Name:      fd.GetName(),
			Type:      ty,
			Ref:       msgRef,
			Tag:       fd.GetNumber(),
			Label:     fd.GetLabel(),
			Alias:     getOption(proto.GetExtension(fd.Options, annotation.E_Alias), ""),
			Bind:      getOption(proto.GetExtension(fd.Options, annotation.E_Bind), annotation.FIELD_BIND_FROM_DEFAULT),
			OmitEmpty: getOption(proto.GetExtension(fd.Options, annotation.E_OmitEmpty), false),
		})
	}
	msg.Fields = fields

	msg.MapEntry = md.Options != nil && md.Options.GetMapEntry()
	msg.Incomplete = false
}

func (p *Parser) parseMethod(md *descriptorpb.MethodDescriptorProto) (*MethodDesc, error) {
	m := &MethodDesc{
		Name:      md.GetName(),
		In:        p.getMessage(md.GetInputType()),
		Out:       p.getMessage(md.GetOutputType()),
		Streaming: md.GetClientStreaming() || md.GetServerStreaming(),
	}
	if opts, ok := proto.GetExtension(md.Options, annotation.E_Http).(*annotation.Http); ok && opts != nil {
		var (
			method string
			path   string
		)
		switch t := opts.Pattern.(type) {
		case *annotation.Http_Get:
			method = "GET"
			path = t.Get
		case *annotation.Http_Post:
			method = "POST"
			path = t.Post
		case *annotation.Http_Put:
			method = "PUT"
			path = t.Put
		case *annotation.Http_Delete:
			method = "DELETE"
			path = t.Delete
		case *annotation.Http_Patch:
			method = "PATCH"
			path = t.Patch
		case *annotation.Http_Custom:
			method = t.Custom.Method
			path = t.Custom.Path
		}
		m.Opts = MethodOptions{
			Method:  method,
			Path:    path,
			Use:     opts.Use,
			Timeout: opts.Timeout,
			Handler: opts.Handler,
		}
	}
	return m, nil
}

func (p *Parser) parseService(sd *descriptorpb.ServiceDescriptorProto) error {
	use, _ := proto.GetExtension(sd.Options, annotation.E_Use).([]string)
	svc := &ServiceDesc{
		Name:     sd.GetName(),
		FullName: normalName(p.prefix + "." + sd.GetName()),
		Opts: ServiceOptions{
			Server:         getOption(proto.GetExtension(sd.Options, annotation.E_Server), ""),
			DefaultHandler: getOption(proto.GetExtension(sd.Options, annotation.E_DefaultHandler), ""),
			DefaultTimeout: getOption(proto.GetExtension(sd.Options, annotation.E_DefaultTimeout), int64(0)),
			PathPrefix:     getOption(proto.GetExtension(sd.Options, annotation.E_PathPrefix), ""),
			Use:            use,
		},
	}
	for _, md := range sd.Method {
		method, err := p.parseMethod(md)
		if err != nil {
			return err
		}
		svc.Methods = append(svc.Methods, method)
	}
	p.svcs = append(p.svcs, svc)
	return nil
}

func (p *Parser) AddFile(fd *descriptorpb.FileDescriptorProto) error {
	p.enter(fd.GetPackage())
	defer p.leave()

	for _, dp := range fd.MessageType {
		p.parseMessage(dp)
	}

	for _, sdp := range fd.Service {
		err := p.parseService(sdp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) GetMessage(name string) *MessageDesc {
	return p.msgs[name]
}

func (p *Parser) Services() []*ServiceDesc {
	return p.svcs
}

func getOption[T comparable](v any, or T) T {
	switch x := v.(type) {
	case T:
		var zero T
		if x != zero {
			return x
		}
	case *T:
		if x != nil {
			return *x
		}
	}
	return or
}
