package annotations

import (
	"encoding/json"
	"strconv"

	"github.com/go-openapi/spec"
)

var builtinMIME = map[string]string{
	"json":                  "application/json",
	"x-www-form-urlencoded": "application/x-www-form-urlencoded",
	"html":                  "text/html",
	"plain":                 "text/plain",
	"xml":                   "text/xml",
}

func getTypeMIME(ty string) string {
	v, ok := builtinMIME[ty]
	if ok {
		return v
	} else {
		return ty
	}
}

func walkOperationResponses(op *spec.Operation, codes []string, f func(resp *spec.Response)) error {
	if op.Responses == nil {
		op.Responses = &spec.Responses{}
	}

	for _, code := range codes {
		if code == "default" {
			if op.Responses.Default == nil {
				op.Responses.Default = &spec.Response{}
			}
			f(op.Responses.Default)
		} else if code == "all" {
			for code, resp := range op.Responses.StatusCodeResponses {
				f(&resp)
				op.Responses.StatusCodeResponses[code] = resp
			}
		} else {
			if op.Responses.StatusCodeResponses == nil {
				op.Responses.StatusCodeResponses = make(map[int]spec.Response)
			}
			statusCode, err := strconv.Atoi(code)
			if err != nil {
				return err
			}
			resp := op.Responses.StatusCodeResponses[statusCode]
			f(&resp)
			op.Responses.StatusCodeResponses[statusCode] = resp
		}
	}
	return nil
}

func parseOperationResponses(op *spec.Operation, tag string, lines []string) error {
	for _, line := range lines {
		fields := ParseLineFields(line, ' ')
		returnCode := GetFieldValue(fields, 0)
		paramType := GetFieldValue(fields, 1)
		dataType := GetFieldValue(fields, 2)
		comment := GetFieldValue(fields, 3)
		walkOperationResponses(op, ParseLineFields(returnCode, ','), func(resp *spec.Response) {
			resp.Description = comment
			resp.Schema = parseAPISchema(paramType, dataType)
		})
	}
	return nil
}

// parseOperationFromAnnotations 通过解析 Annotations 构造 spec.Operation。
// 大部分格式参考了 swaggo 的注解(https://pkg.go.dev/github.com/swaggo/swag@v1.16.1#readme-api-operation)，去掉了一些不好实现的部分。
func ParseOperationFromAnnotations(id string, ans Annotations) (*spec.Operation, error) {
	op := spec.NewOperation(id).
		WithSummary(ans.Get("summary").Text()).
		WithDescription(ans.Get("description").Text()).
		WithTags(ParseLineFields(ans.Get("tags").Line(-1), ',')...)

	if accept := ans.Get("accept"); accept != nil {
		for _, v := range accept.lines {
			op.Consumes = append(op.Consumes, getTypeMIME(v))
		}
	}
	if produce := ans.Get("produce"); produce != nil {
		for _, v := range produce.lines {
			op.Produces = append(op.Produces, getTypeMIME(v))
		}
	}

	if params := ans.Get("param"); params != nil {
		for _, line := range params.lines {
			fields := ParseLineFields(line, ' ')
			if len(fields) < 3 {
				continue
			}
			paramName := GetFieldValue(fields, 0)
			paramType := GetFieldValue(fields, 1)
			dataType := GetFieldValue(fields, 2)
			isMandatory := GetFieldValue(fields, 3) == "true"
			comment := GetFieldValue(fields, 4)
			op.Parameters = append(op.Parameters, spec.Parameter{
				ParamProps: spec.ParamProps{
					Description: comment,
					Name:        paramName,
					In:          paramType,
					Required:    isMandatory,
					Schema:      parseObjectSchema(dataType),
				},
			})
		}
	}

	if headers := ans.Get("header"); headers != nil {
		for _, line := range headers.lines {
			fields := ParseLineFields(line, ' ')
			returnCode := GetFieldValue(fields, 0)
			paramType := GetFieldValue(fields, 1)
			name := GetFieldValue(fields, 2)
			comment := GetFieldValue(fields, 3)
			walkOperationResponses(op, ParseLineFields(returnCode, ','), func(resp *spec.Response) {
				if resp.Headers == nil {
					resp.Headers = make(map[string]spec.Header)
				}
				resp.Headers[name] = spec.Header{
					SimpleSchema: spec.SimpleSchema{
						Type: paramType,
					},
					HeaderProps: spec.HeaderProps{
						Description: comment,
					},
				}
			})
		}
	}

	if success := ans.Get("success"); success != nil {
		err := parseOperationResponses(op, "success", success.lines)
		if err != nil {
			return nil, err
		}
	}
	if failure := ans.Get("failure"); failure != nil {
		err := parseOperationResponses(op, "failure", failure.lines)
		if err != nil {
			return nil, err
		}
	}
	if response := ans.Get("response"); response != nil {
		err := parseOperationResponses(op, "response", response.lines)
		if err != nil {
			return nil, err
		}
	}

	if security := ans.Get("security"); security != nil {
		for _, line := range security.lines {
			if line == "" {
				continue
			}
			var sec map[string][]string
			err := json.Unmarshal([]byte(line), &sec)
			if err != nil {
				return nil, err
			}
			op.Security = append(op.Security, sec)
		}
	}

	op.Deprecated = ans.Get("deprecated") != nil

	return op, nil
}
