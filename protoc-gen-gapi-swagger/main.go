package main

import (
	"flag"

	"github.com/vizee/gapi-plus/protoc-gen-gapi-swagger/gapi"
	"github.com/vizee/gapi-plus/protoc-gen-gapi-swagger/gen"
	"google.golang.org/protobuf/compiler/protogen"
)

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
	}, nil).Run)
}
