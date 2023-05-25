package gen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/vizee/gapi-plus/protoc-gen-gapi-swagger/annotations"
	"github.com/vizee/gapi-proto-go/gapi"
	gapiproto "github.com/vizee/gapi-proto-go/gapi"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MethodHandler func(method *protogen.Method, annotations annotations.Annotations, operation *spec.Operation)

type Config struct {
	Out                *string
	Template           *string
	HandlerTemplate    *string
	MiddlewareTemplate *string

	Global      MethodHandler
	Handlers    map[string]MethodHandler
	Middlewares map[string]MethodHandler
}

type Generator struct {
	conf         *Config
	doc          *spec.Swagger
	handleGlobal MethodHandler
	handlers     map[string]MethodHandler
	middlewares  map[string]MethodHandler

	visit map[string]bool
}

func (g *Generator) parseEnum(enum *protogen.Enum) error {
	if g.visit[string(enum.Desc.FullName())] {
		return nil
	}
	g.visit[string(enum.Desc.FullName())] = true

	schema := spec.Int32Property()

	var desc strings.Builder
	comments := strings.TrimSpace(string(enum.Comments.Leading))
	if comments != "" {
		desc.WriteString(comments)
	}
	for _, ev := range enum.Values {
		v := int32(ev.Desc.Number())
		schema.Enum = append(schema.Enum, v)
		if desc.Len() > 0 {
			desc.WriteByte('\n')
		}
		fmt.Fprintf(&desc, "%d: %s", v, ev.Desc.Name())
		comments := strings.TrimSpace(string(enum.Comments.Leading))
		if comments != "" {
			desc.WriteString(" // ")
			desc.WriteString(comments)
		}
	}
	schema.Description = desc.String()

	g.doc.Definitions[string(enum.Desc.FullName())] = *schema
	return nil
}

func (g *Generator) parseField(field protoreflect.FieldDescriptor) (*spec.Schema, error) {
	if field.IsMap() {
		prop, err := g.parseField(field.MapValue())
		if err != nil {
			return nil, err
		}
		return spec.MapProperty(prop), nil
	}
	var prop *spec.Schema
	switch field.Kind() {
	case protoreflect.BoolKind:
		prop = spec.BooleanProperty()
	case protoreflect.EnumKind:
		prop = spec.RefProperty(fmt.Sprintf("#/definitions/%s", field.Enum().FullName()))
	case protoreflect.Int32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Uint32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Fixed32Kind:
		prop = spec.Int32Property()
	case protoreflect.Int64Kind,
		protoreflect.Sint64Kind,
		protoreflect.Uint64Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.Fixed64Kind:
		prop = spec.Int64Property()
	case protoreflect.FloatKind:
		prop = spec.Float32Property()
	case protoreflect.DoubleKind:
		prop = spec.Float64Property()
	case protoreflect.StringKind:
		prop = spec.StringProperty()
	case protoreflect.BytesKind:
		prop = spec.StrFmtProperty("byte")
	case protoreflect.MessageKind:
		prop = spec.RefProperty(fmt.Sprintf("#/definitions/%s", field.Message().FullName()))
	default:
		return nil, fmt.Errorf("unsupported kind %s", field.Kind())
	}
	if field.IsList() {
		prop = spec.ArrayProperty(prop)
	}
	return prop, nil
}

func (g *Generator) parseMessage(msg *protogen.Message) error {
	if msg.Desc.IsMapEntry() || g.visit[string(msg.Desc.FullName())] {
		return nil
	}
	g.visit[string(msg.Desc.FullName())] = true

	for _, nested := range msg.Enums {
		err := g.parseEnum(nested)
		if err != nil {
			return err
		}
	}

	for _, nested := range msg.Messages {
		err := g.parseMessage(nested)
		if err != nil {
			return err
		}
	}

	schema := spec.Schema{
		SchemaProps: spec.SchemaProps{
			ID:          string(msg.Desc.FullName()),
			Type:        []string{"object"},
			Description: strings.TrimSpace(string(msg.Comments.Leading)),
			Properties:  make(spec.SchemaProperties, len(msg.Fields)),
		},
	}

	for _, field := range msg.Fields {
		prop, err := g.parseField(field.Desc)
		if err != nil {
			return err
		}
		prop.Description = strings.TrimSpace(string(field.Comments.Leading))

		bind, ok := proto.GetExtension(field.Desc.Options(), gapiproto.E_Bind).(gapiproto.FIELD_BIND)
		if ok && bind != gapi.FIELD_BIND_FROM_DEFAULT {
			if prop.Description != "" {
				prop.Description += " [" + bind.String() + "]"
			} else {
				prop.Description = "[" + bind.String() + "]"
			}
		}

		name := field.Desc.TextName()
		alias, _ := proto.GetExtension(field.Desc.Options(), gapiproto.E_Alias).(string)
		if alias != "" {
			name = alias
		}
		schema.Properties[name] = *prop
	}

	if g.doc.Definitions == nil {
		g.doc.Definitions = make(spec.Definitions)
	}
	g.doc.Definitions[schema.ID] = schema

	return nil
}

func (g *Generator) parseService(service *protogen.Service) error {
	serviceOpts := service.Desc.Options()
	serverName, _ := proto.GetExtension(serviceOpts, gapiproto.E_Server).(string)
	if serverName == "" {
		return nil
	}

	defaultHandler, _ := proto.GetExtension(serviceOpts, gapiproto.E_DefaultHandler).(string)
	commonUse, _ := proto.GetExtension(serviceOpts, gapiproto.E_Use).([]string)
	pathPrefix, _ := proto.GetExtension(serviceOpts, gapiproto.E_PathPrefix).(string)

	serviceAns := annotations.ExtractAnnotations(string(service.Comments.Leading))
	serviceTags := annotations.ParseLineFields(serviceAns.Get("tags").Line(-1), ',')

	for _, method := range service.Methods {
		httpOpt, _ := proto.GetExtension(method.Desc.Options(), gapiproto.E_Http).(*gapi.Http)
		if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() || httpOpt == nil {
			continue
		}

		err := g.parseMessage(method.Input)
		if err != nil {
			return err
		}
		err = g.parseMessage(method.Output)
		if err != nil {
			return err
		}

		methodAns := annotations.ExtractAnnotations(string(method.Comments.Leading))

		op, err := annotations.ParseOperationFromAnnotations(string(method.Desc.FullName()), methodAns)
		if err != nil {
			return err
		}
		op.Tags = append(op.Tags, serviceTags...)

		gf := g.handleGlobal
		if gf != nil {
			gf(method, methodAns, op)
		}

		handler := httpOpt.Handler
		if handler == "" {
			handler = defaultHandler
		}

		hf := g.handlers[handler]
		if hf != nil {
			hf(method, methodAns, op)
		}

		for _, use := range commonUse {
			mf := g.middlewares[use]
			if mf != nil {
				mf(method, methodAns, op)
			}
		}
		for _, use := range httpOpt.Use {
			mf := g.middlewares[use]
			if mf != nil {
				mf(method, methodAns, op)
			}
		}

		var (
			method string
			path   string
		)
		switch t := httpOpt.Pattern.(type) {
		case *gapiproto.Http_Get:
			method = "GET"
			path = t.Get
		case *gapiproto.Http_Post:
			method = "POST"
			path = t.Post
		case *gapiproto.Http_Put:
			method = "PUT"
			path = t.Put
		case *gapiproto.Http_Delete:
			method = "DELETE"
			path = t.Delete
		case *gapiproto.Http_Patch:
			method = "PATCH"
			path = t.Patch
		case *gapiproto.Http_Custom:
			method = t.Custom.Method
			path = t.Custom.Path
		}

		pathItem := g.doc.Paths.Paths[pathPrefix+path]
		switch method {
		case "GET":
			pathItem.Get = op
		case "PUT":
			pathItem.Put = op
		case "POST":
			pathItem.Post = op
		case "DELETE":
			pathItem.Delete = op
		case "OPTIONS":
			pathItem.Options = op
		case "HEAD":
			pathItem.Head = op
		case "PATCH":
			pathItem.Patch = op
		}
		g.doc.Paths.Paths[pathPrefix+path] = pathItem
	}

	return nil
}

func (g *Generator) Run(plugin *protogen.Plugin) error {
	if f := *g.conf.Template; f != "" {
		err := loadJsonFile(f, g.doc)
		if err != nil {
			return err
		}
	}
	if f := *g.conf.HandlerTemplate; f != "" {
		tmpls, err := LoadHandlerTemplate(f)
		if err != nil {
			return err
		}
		if g.conf.Handlers == nil {
			g.conf.Handlers = make(map[string]MethodHandler)
		}
		mergeMap(g.conf.Handlers, tmpls)
	}
	if f := *g.conf.MiddlewareTemplate; f != "" {
		tmpls, err := LoadMiddlewareTemplate(f)
		if err != nil {
			return err
		}
		if g.conf.Middlewares == nil {
			g.conf.Middlewares = make(map[string]MethodHandler)
		}
		mergeMap(g.conf.Middlewares, tmpls)
	}

	if g.doc.Paths == nil {
		g.doc.Paths = &spec.Paths{
			Paths: make(map[string]spec.PathItem),
		}
	}

	for _, f := range plugin.Files {
		if !f.Generate {
			continue
		}

		for _, enum := range f.Enums {
			err := g.parseEnum(enum)
			if err != nil {
				return err
			}
		}

		for _, msg := range f.Messages {
			err := g.parseMessage(msg)
			if err != nil {
				return err
			}
		}

		for _, svc := range f.Services {
			err := g.parseService(svc)
			if err != nil {
				return err
			}
		}
	}

	gf := plugin.NewGeneratedFile(*g.conf.Out, "")

	switch {
	case strings.HasSuffix(*g.conf.Out, ".yaml"):
		data, err := marshalJsonToYaml(g.doc)
		if err != nil {
			return err
		}
		gf.P("# Code generated by protoc-gen-gapi-swagger. DO NOT EDIT.")
		_, _ = gf.Write(data)

	case strings.HasSuffix(*g.conf.Out, ".json"):
		data, err := json.MarshalIndent(g.doc, "", "  ")
		if err != nil {
			return err
		}
		_, _ = gf.Write(data)
		gf.P()
	}

	return nil
}

func NewGenerator(conf *Config, doc *spec.Swagger) *Generator {
	if doc == nil {
		doc = &spec.Swagger{
			SwaggerProps: spec.SwaggerProps{
				Swagger: "2.0",
			},
		}
	}
	return &Generator{
		conf:         conf,
		doc:          doc,
		handleGlobal: conf.Global,
		handlers:     conf.Handlers,
		middlewares:  conf.Middlewares,
		visit:        make(map[string]bool),
	}
}
