package apidesc

import (
	"errors"
	"time"

	"github.com/vizee/gapi-plus/apimeta/internal/helpers"
	"github.com/vizee/gapi-plus/apimeta/internal/slices"
	"github.com/vizee/gapi-plus/proto/descriptor"
	annotation "github.com/vizee/gapi-proto-go/gapi"
	"github.com/vizee/gapi/metadata"
	"github.com/vizee/jsonpb"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ResolvingCache struct {
	msgs map[string]*helpers.Message
}

func (rc *ResolvingCache) resolveMessage(md *descriptor.MessageDesc) *helpers.Message {
	if rc.msgs == nil {
		rc.msgs = make(map[string]*helpers.Message)
	}

	msg := rc.msgs[md.Name]
	if msg != nil {
		return msg
	}

	msg = &helpers.Message{
		Message: &jsonpb.Message{
			Name:   md.Name,
			Fields: make([]jsonpb.Field, 0, len(md.Fields)),
		},
	}
	// 防止递归
	rc.msgs[msg.Name] = msg

	for _, fd := range md.Fields {
		kind, ok := helpers.GetTypeKind(fd.Type)
		if !ok {
			continue
		}
		name := fd.Name
		if fd.Alias != "" {
			name = fd.Alias
		}

		if fd.Bind == annotation.FIELD_BIND_FROM_DEFAULT {
			repeated := fd.Label == descriptorpb.FieldDescriptorProto_LABEL_REPEATED
			var msgRef *jsonpb.Message
			if kind == jsonpb.MessageKind {
				msgRef = rc.resolveMessage(fd.Ref).Message
				if fd.Ref.MapEntry {
					kind = jsonpb.MapKind
					repeated = false
				}
			}
			omit := jsonpb.OmitProtoEmpty
			if fd.OmitEmpty {
				omit = jsonpb.OmitEmpty
			}
			msg.Fields = append(msg.Fields, jsonpb.Field{
				Name:     name,
				Kind:     kind,
				Ref:      msgRef,
				Tag:      uint32(fd.Tag),
				Repeated: repeated,
				Omit:     omit,
			})
		} else {
			var bind metadata.BindSource
			switch fd.Bind {
			case annotation.FIELD_BIND_FROM_QUERY:
				bind = metadata.BindQuery
			case annotation.FIELD_BIND_FROM_PARAMS:
				bind = metadata.BindParams
			case annotation.FIELD_BIND_FROM_HEADER:
				bind = metadata.BindHeader
			case annotation.FIELD_BIND_FROM_CONTEXT:
				bind = metadata.BindContext
			}
			msg.Bindings = append(msg.Bindings, metadata.FieldBinding{
				Name: name,
				Kind: kind,
				Tag:  uint32(fd.Tag),
				Bind: bind,
			})
		}
	}

	msg.Fields = slices.Shrink(msg.Fields)
	msg.BakeTagIndex()
	msg.BakeNameIndex()

	msg.Bindings = slices.Shrink(msg.Bindings)

	return msg
}

func ResolveRoutes(rc *ResolvingCache, sds []*descriptor.ServiceDesc, ignoreError bool) ([]*metadata.Route, error) {
	routesNum := 0
	for _, sd := range sds {
		if sd.Opts.Server == "" {
			continue
		}
		for _, md := range sd.Methods {
			if md.Streaming || md.Opts.Method == "" || md.Opts.Path == "" {
				continue
			}
			routesNum++
		}
	}
	routes := make([]*metadata.Route, 0, routesNum)
walksd:
	for _, sd := range sds {
		server := sd.Opts.Server
		if server == "" {
			if ignoreError {
				continue
			}
			return nil, errors.New("invalid service '" + sd.Name + "'")
		}
		for _, use := range sd.Opts.Use {
			if !helpers.CheckMiddlewareName(use) {
				if ignoreError {
					continue walksd
				}
				return nil, errors.New("invalid middleware name '" + use + "'")
			}
		}

	walkmd:
		for _, md := range sd.Methods {
			for _, use := range md.Opts.Use {
				if !helpers.CheckMiddlewareName(use) {
					if ignoreError {
						continue walkmd
					}
					return nil, errors.New("invalid middleware name '" + use + "'")
				}
			}

			handler := md.Opts.Handler
			if handler == "" {
				handler = sd.Opts.DefaultHandler
			}
			if handler == "" || md.Streaming || md.Opts.Method == "" || md.Opts.Path == "" || md.In == nil || md.In.Incomplete || md.Out == nil || md.Out.Incomplete {
				if ignoreError {
					continue
				}
				return nil, errors.New("invalid method '" + md.Name + "'")
			}

			timeout := md.Opts.Timeout
			if timeout == 0 {
				timeout = sd.Opts.DefaultTimeout
			}

			inMsg := rc.resolveMessage(md.In)
			routes = append(routes, &metadata.Route{
				Method: md.Opts.Method,
				Path:   sd.Opts.PathPrefix + md.Opts.Path,
				Use:    slices.Merge(sd.Opts.Use, md.Opts.Use),
				Call: &metadata.Call{
					Server:   server,
					Handler:  handler,
					Method:   helpers.ConcatFullMethodName(sd.FullName, md.Name),
					In:       inMsg.Message,
					Out:      rc.resolveMessage(md.Out).Message,
					Bindings: inMsg.Bindings,
					Timeout:  time.Duration(timeout) * time.Millisecond,
				},
			})
		}
	}

	return routes, nil
}
