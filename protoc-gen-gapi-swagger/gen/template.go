package gen

import (
	"github.com/go-openapi/spec"
	"github.com/vizee/gapi-plus/protoc-gen-gapi-swagger/annotations"
	"google.golang.org/protobuf/compiler/protogen"
)

func mergeResponses(dst *spec.Responses, src *spec.Responses) {
	if src.Default != nil {
		dst.Default = src.Default
	}
	if dst.StatusCodeResponses == nil {
		dst.StatusCodeResponses = make(map[int]spec.Response)
	}
	mergeMap(dst.StatusCodeResponses, src.StatusCodeResponses)
}

type HandlerTemplate struct {
	Consumes   []string         `json:"consumes"`
	Produces   []string         `json:"produces"`
	Parameters []spec.Parameter `json:"parameters"`
	Responses  *spec.Responses  `json:"responses"`
}

func (ht *HandlerTemplate) Handle(method *protogen.Method, annotations annotations.Annotations, operation *spec.Operation) {
	operation.Consumes = append(operation.Consumes, ht.Consumes...)
	operation.Produces = append(operation.Produces, ht.Produces...)
	operation.Parameters = append(operation.Parameters, ht.Parameters...)
	if ht.Responses != nil {
		if operation.Responses == nil {
			operation.Responses = &spec.Responses{}
		}
		mergeResponses(operation.Responses, ht.Responses)
	}
}

func LoadHandlerTemplate(fname string) (map[string]MethodHandler, error) {
	var tmpls map[string]*HandlerTemplate
	err := loadJsonFile(fname, &tmpls)
	if err != nil {
		return nil, err
	}
	handlers := make(map[string]MethodHandler, len(tmpls))
	for k, hi := range tmpls {
		handlers[k] = hi.Handle
	}
	return handlers, nil
}

type MiddlewareTemplate struct {
	Parameters []spec.Parameter      `json:"parameters"`
	Security   []map[string][]string `json:"security"`
	Responses  *spec.Responses       `json:"responses"`
}

func (mt *MiddlewareTemplate) Handle(method *protogen.Method, annotations annotations.Annotations, operation *spec.Operation) {
	operation.Parameters = append(operation.Parameters, mt.Parameters...)
	operation.Security = append(operation.Security, mt.Security...)
	if mt.Responses != nil {
		if operation.Responses == nil {
			operation.Responses = &spec.Responses{}
		}
		mergeResponses(operation.Responses, mt.Responses)
	}
}

func LoadMiddlewareTemplate(fname string) (map[string]MethodHandler, error) {
	var tmpls map[string]*MiddlewareTemplate
	err := loadJsonFile(fname, &tmpls)
	if err != nil {
		return nil, err
	}
	handlers := make(map[string]MethodHandler, len(tmpls))
	for k, hi := range tmpls {
		handlers[k] = hi.Handle
	}
	return handlers, nil
}
