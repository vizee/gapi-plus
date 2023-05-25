package gapi

import (
	"fmt"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/vizee/gapi-plus/protoc-gen-gapi-swagger/annotations"
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

func JsonAPI(method *protogen.Method, annotations annotations.Annotations, operation *spec.Operation) {
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

	schema := spec.RefSchema(fmt.Sprintf("#/definitions/%s", method.Output.Desc.FullName()))

	var fieldName string
	out := annotations.Get("jsonapi.out").Line(0)
	slash := strings.LastIndexByte(out, '/')
	if slash >= 0 {
		refType := out[:slash]
		fieldName = out[slash+1:]
		ref := spec.RefSchema(fmt.Sprintf("#/definitions/%s", refType))
		schema = spec.ComposedSchema(*ref, spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: spec.SchemaProperties{
					fieldName: *schema,
				},
			},
		})
	}

	operation.Responses.StatusCodeResponses[200] = *spec.NewResponse().WithSchema(schema)
}
