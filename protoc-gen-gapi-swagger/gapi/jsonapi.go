package gapi

import (
	"fmt"

	"github.com/go-openapi/spec"
	"github.com/vizee/gapi-plus/protoc-gen-gapi-swagger/gen"
	"google.golang.org/protobuf/compiler/protogen"
)

func addToSet(s []string, v string) []string {
	found := false
	for _, t := range s {
		if t == v {
			found = true
			break
		}
	}
	if !found {
		s = append(s, v)
	}
	return s
}

func JsonAPI(method *protogen.Method, annotations gen.Annotations, operation *spec.Operation) {
	const jsonType = "application/json"
	operation.Consumes = addToSet(operation.Consumes, jsonType)
	operation.Produces = addToSet(operation.Produces, jsonType)

	operation.Parameters = append(operation.Parameters, spec.Parameter{
		ParamProps: spec.ParamProps{
			In:       "body",
			Required: true,
			Schema:   spec.RefSchema(fmt.Sprintf("#/definitions/%s", method.Input.Desc.FullName())),
		},
	})

	if operation.Responses == nil {
		operation.Responses = &spec.Responses{}
	}
	if operation.Responses.StatusCodeResponses == nil {
		operation.Responses.StatusCodeResponses = make(map[int]spec.Response)
	}
	operation.Responses.StatusCodeResponses[200] = *spec.NewResponse().WithSchema(spec.RefSchema(fmt.Sprintf("#/definitions/%s", method.Output.Desc.FullName())))
}
