package main

import (
	"flag"

	"github.com/go-openapi/spec"
	"github.com/vizee/gapi-plus/protoc-gen-gapi-swagger/gapi"
	"github.com/vizee/gapi-plus/protoc-gen-gapi-swagger/gen"
	gapiproto "github.com/vizee/gapi-proto-go/gapi"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func fieldBindInfo(field *protogen.Field, prop *spec.Schema) bool {
	bind, ok := proto.GetExtension(field.Desc.Options(), gapiproto.E_Bind).(gapiproto.FIELD_BIND)
	if ok && bind != gapiproto.FIELD_BIND_FROM_DEFAULT {
		if prop.Description != "" {
			prop.Description += " [" + bind.String() + "]"
		} else {
			prop.Description = "[" + bind.String() + "]"
		}
	}
	return true
}

func main() {
	var flags flag.FlagSet
	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(gen.NewGenerator(&gen.Config{
		Out:                flags.String("out", "swagger.yaml", "output file"),
		Template:           flags.String("template", "", "swagger template json file"),
		HandlerTemplate:    flags.String("handlers", "", "handlers template json file"),
		MiddlewareTemplate: flags.String("middlewares", "", "middlewares template json file"),
		Handlers:           map[string]gen.MethodHandler{"jsonapi": gapi.JsonAPI},
		HandleField:        fieldBindInfo,
	}, nil).Run)
}
