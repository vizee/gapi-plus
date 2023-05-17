package protodesc

import (
	"errors"
	"strings"
	"time"

	"github.com/vizee/gapi-plus/apimeta/internal/helpers"
	"github.com/vizee/gapi-plus/apimeta/internal/slices"
	annotation "github.com/vizee/gapi-proto-go/gapi"
	"github.com/vizee/gapi/metadata"
	"github.com/vizee/jsonpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Parser struct {
	ns     []string
	prefix string
	msgs   map[string]*helpers.Message
}

func NewParser() *Parser {
	return &Parser{
		msgs: make(map[string]*helpers.Message),
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

func (p *Parser) getMessage(fullName string) *helpers.Message {
	msg := p.msgs[fullName]
	if msg == nil {
		msg = &helpers.Message{
			Message: &jsonpb.Message{
				Name: normalName(fullName),
			},
			Incomplete: true,
		}
		p.msgs[fullName] = msg
	}
	return msg
}

func (p *Parser) parseMessage(md *descriptorpb.DescriptorProto) error {
	p.enter(md.GetName())
	defer p.leave()

	fullName := p.prefix
	msg := p.getMessage(fullName)
	if !msg.Incomplete {
		return errors.New(fullName + " message has been parsed")
	}

	for _, nested := range md.NestedType {
		err := p.parseMessage(nested)
		if err != nil {
			return err
		}
	}

	fields := make([]jsonpb.Field, 0, len(md.Field))
	var bindings []metadata.FieldBinding
	for _, fd := range md.Field {
		ty := fd.GetType()
		kind, ok := helpers.GetTypeKind(ty)
		if !ok {
			continue
		}

		name := fd.GetName()
		alias := getOption(proto.GetExtension(fd.Options, annotation.E_Alias), "")
		if alias != "" {
			name = alias
		}

		bind := getOption(proto.GetExtension(fd.Options, annotation.E_Bind), annotation.FIELD_BIND_FROM_DEFAULT)

		if bind == annotation.FIELD_BIND_FROM_DEFAULT {
			repeated := fd.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED

			var msgRef *jsonpb.Message
			if kind == jsonpb.MessageKind {
				refName := fd.GetTypeName()
				if !strings.HasPrefix(refName, ".") {
					// 这里只搜索了嵌套 scope，按照标准实现应该向上搜索到根
					refName = p.prefix + "." + refName
				}

				ref := p.getMessage(refName)
				msgRef = ref.Message
				// map entry 一般从 nested 提供，不需要推迟处理
				if repeated && ref.MapEntry {
					repeated = false
				}
			}

			omit := jsonpb.OmitProtoEmpty
			omitEmpty := getOption(proto.GetExtension(fd.Options, annotation.E_OmitEmpty), false)
			if omitEmpty {
				omit = jsonpb.OmitEmpty
			}

			fields = append(fields, jsonpb.Field{
				Name:     name,
				Kind:     kind,
				Ref:      msgRef,
				Tag:      uint32(fd.GetNumber()),
				Repeated: repeated,
				Omit:     omit,
			})
		} else {
			var bindSource metadata.BindSource
			switch bind {
			case annotation.FIELD_BIND_FROM_QUERY:
				bindSource = metadata.BindQuery
			case annotation.FIELD_BIND_FROM_PARAMS:
				bindSource = metadata.BindParams
			case annotation.FIELD_BIND_FROM_HEADER:
				bindSource = metadata.BindHeader
			case annotation.FIELD_BIND_FROM_CONTEXT:
				bindSource = metadata.BindContext
			}
			bindings = append(bindings, metadata.FieldBinding{
				Name: name,
				Kind: kind,
				Tag:  uint32(fd.GetNumber()),
				Bind: bindSource,
			})
		}
	}

	msg.Fields = slices.Shrink(fields)
	msg.BakeNameIndex()
	msg.BakeTagIndex()

	msg.Bindings = slices.Shrink(bindings)
	msg.MapEntry = md.Options != nil && md.Options.GetMapEntry()
	msg.Incomplete = false

	return nil
}

func (p *Parser) parseService(routes []*metadata.Route, sd *descriptorpb.ServiceDescriptorProto, ignoreError bool) ([]*metadata.Route, error) {
	server := getOption(proto.GetExtension(sd.Options, annotation.E_Server), "")
	if server == "" {
		if ignoreError {
			return routes, nil
		}
		return nil, errors.New("invalid service name '" + sd.GetName() + "'")
	}

	commonUses, _ := proto.GetExtension(sd.Options, annotation.E_Use).([]string)
	for _, use := range commonUses {
		if !helpers.CheckMiddlewareName(use) {
			if ignoreError {
				return routes, nil
			}
			return nil, errors.New("invalid middleware name '" + use + "'")
		}
	}

	defaultHandler := getOption(proto.GetExtension(sd.Options, annotation.E_DefaultHandler), "")
	defaultTimeout := getOption(proto.GetExtension(sd.Options, annotation.E_DefaultTimeout), int64(0))
	pathPrefix := getOption(proto.GetExtension(sd.Options, annotation.E_PathPrefix), "")

walkmd:
	for _, md := range sd.Method {
		httpOpt, ok := proto.GetExtension(md.Options, annotation.E_Http).(*annotation.Http)
		if !ok || httpOpt == nil {
			continue
		}

		for _, use := range httpOpt.Use {
			if !helpers.CheckMiddlewareName(use) {
				if ignoreError {
					continue walkmd
				}
				return nil, errors.New("invalid middleware name '" + use + "'")
			}
		}

		handler := httpOpt.Handler
		if handler == "" {
			handler = defaultHandler
		}
		var (
			method string
			path   string
		)
		switch t := httpOpt.Pattern.(type) {
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

		if handler == "" || method == "" || path == "" || md.GetClientStreaming() || md.GetServerStreaming() {
			if ignoreError {
				continue
			}
			return nil, errors.New("invalid method '" + md.GetName() + "'")
		}

		timeout := httpOpt.Timeout
		if timeout == 0 {
			timeout = defaultTimeout
		}

		sd.GetName()
		serviceFullname := normalName(p.prefix + "." + sd.GetName())

		inMsg := p.getMessage(md.GetInputType())
		routes = append(routes, &metadata.Route{
			Method: method,
			Path:   pathPrefix + path,
			Use:    slices.Merge(commonUses, httpOpt.Use),
			Call: &metadata.Call{
				Server:   server,
				Handler:  handler,
				Method:   helpers.ConcatFullMethodName(serviceFullname, md.GetName()),
				In:       inMsg.Message,
				Out:      p.getMessage(md.GetOutputType()).Message,
				Bindings: inMsg.Bindings,
				Timeout:  time.Duration(timeout) * time.Millisecond,
			},
		})
	}

	return routes, nil
}

func (p *Parser) AddFile(routes []*metadata.Route, fd *descriptorpb.FileDescriptorProto, ignoreError bool) ([]*metadata.Route, error) {
	p.enter(fd.GetPackage())
	defer p.leave()

	for _, dp := range fd.MessageType {
		err := p.parseMessage(dp)
		if err != nil {
			return nil, err
		}
	}

	for _, sd := range fd.Service {
		var err error
		routes, err = p.parseService(routes, sd, ignoreError)
		if err != nil {
			return nil, err
		}
	}

	return routes, nil
}

func (p *Parser) CheckIncomplete() []string {
	var incomplete []string
	for _, m := range p.msgs {
		if m.Incomplete {
			incomplete = append(incomplete, m.Name)
		}
	}
	return incomplete
}

func normalName(name string) string {
	if len(name) > 0 && name[0] == '.' {
		return name[1:]
	}
	return name
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
