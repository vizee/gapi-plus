package gen

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-openapi/spec"
)

type Annotation struct {
	name  string
	lines []string
}

func (a *Annotation) Line(i int) string {
	if a != nil {
		if i < 0 {
			return strings.Join(a.lines, " ")
		} else if i < len(a.lines) {
			return a.lines[i]
		}
	}
	return ""
}

func (a *Annotation) LineNum() int {
	if a != nil {
		return len(a.lines)
	}
	return 0
}

func (a *Annotation) Text() string {
	if a != nil {
		return strings.Join(a.lines, "\n")
	}
	return ""
}

type Annotations []*Annotation

func (ans Annotations) Get(name string) *Annotation {
	for _, an := range ans {
		if an.name == name {
			return an
		}
	}
	return nil
}

func extractAnnotations(comments string) Annotations {
	comments = strings.TrimSpace(comments)
	if comments == "" {
		return nil
	}

	var (
		curr *Annotation
		ans  Annotations
	)
	for _, line := range strings.Split(comments, "\n") {
		line = strings.TrimSpace(line)

		var name string
		if strings.HasPrefix(line, "@") {
			sp := strings.IndexByte(line, ' ')
			if sp < 0 {
				sp = len(line)
			}
			name = line[1:sp]
			line = strings.TrimSpace(line[sp:])
		}

		name = strings.ToLower(name)
		if curr == nil {
			if name == "" {
				continue
			}
			curr = ans.Get(name)
		}
		if curr == nil || (name != "" && curr.name != name) {
			curr = &Annotation{
				name: name,
			}
			ans = append(ans, curr)
		}

		curr.lines = append(curr.lines, line)
	}

	return ans
}

func nextField(line string, sep byte) (string, int) {
	i := 0
	for i < len(line) {
		c := line[i]
		if c == '"' {
			i++
			for i < len(line) && (line[i] != '"' || line[i-1] == '\\') {
				i++
			}
		} else if c == sep {
			break
		}
		i++
	}
	return strings.TrimSpace(line[:i]), i + 1
}

func parseLineFields(line string, sep byte) []string {
	if len(line) == 0 {
		return nil
	}
	spaceSep := unicode.IsSpace(rune(sep))
	i := 0
	var fields []string
	for i < len(line) {
		field, n := nextField(line[i:], sep)
		if field != "" || !spaceSep {
			fields = append(fields, field)
		}
		i += n
	}
	return fields
}

func getFieldValue(fields []string, i int) string {
	if i < len(fields) {
		v := fields[i]
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			u, err := strconv.Unquote(v)
			if err == nil {
				v = u
			}
		}
		return v
	}
	return ""
}

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

var builtinType = map[string]*spec.Schema{
	"boolean":   spec.BoolProperty(),
	"string":    spec.StringProperty(),
	"double":    spec.Float64Property(),
	"float":     spec.Float32Property(),
	"int8":      spec.Int8Property(),
	"int16":     spec.Int16Property(),
	"int32":     spec.Int32Property(),
	"int64":     spec.Int64Property(),
	"date":      spec.DateProperty(),
	"date-time": spec.DateTimeProperty(),
}

func getTypeScheme(name string) *spec.Schema {
	if strings.HasPrefix(name, "[]") {
		return spec.ArrayProperty(getTypeScheme(name[2:]))
	}
	ty := builtinType[name]
	if ty != nil {
		return ty
	}
	return spec.RefSchema(fmt.Sprintf("#/definitions/%s", name))
}

func getResponseScheme(paramType string, dataType string) *spec.Schema {
	paramType = strings.TrimPrefix(strings.TrimSuffix(paramType, "}"), "{")

	switch paramType {
	case "object":
		if dataType != "" {
			return spec.RefSchema(fmt.Sprintf("#/definitions/%s", dataType))
		} else {
			return &spec.Schema{SchemaProps: spec.SchemaProps{Type: []string{"object"}}}
		}
	case "array":
		return spec.ArrayProperty(getTypeScheme(dataType))
	default:
		return &spec.Schema{SchemaProps: spec.SchemaProps{Type: []string{paramType}, Format: dataType}}
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
		fields := parseLineFields(line, ' ')
		returnCode := getFieldValue(fields, 0)
		paramType := getFieldValue(fields, 1)
		dataType := getFieldValue(fields, 2)
		comment := getFieldValue(fields, 3)
		walkOperationResponses(op, parseLineFields(returnCode, ','), func(resp *spec.Response) {
			resp.Description = comment
			resp.Schema = getResponseScheme(paramType, dataType)
		})
	}
	return nil
}

// parseOperationFromAnnotations 通过解析 Annotations 构造 spec.Operation。
// 大部分格式参考了 swaggo 的注解(https://pkg.go.dev/github.com/swaggo/swag@v1.16.1#readme-api-operation)，去掉了一些不好实现的部分。
func parseOperationFromAnnotations(id string, ans Annotations) (*spec.Operation, error) {
	op := spec.NewOperation(id).
		WithSummary(ans.Get("summary").Text()).
		WithDescription(ans.Get("description").Text()).
		WithTags(parseLineFields(ans.Get("tags").Line(-1), ',')...)

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
			fields := parseLineFields(line, ' ')
			if len(fields) < 3 {
				continue
			}
			paramName := getFieldValue(fields, 0)
			paramType := getFieldValue(fields, 1)
			dataType := getFieldValue(fields, 2)
			isMandatory := getFieldValue(fields, 3) == "true"
			comment := getFieldValue(fields, 4)
			op.Parameters = append(op.Parameters, spec.Parameter{
				ParamProps: spec.ParamProps{
					Description: comment,
					Name:        paramName,
					In:          paramType,
					Required:    isMandatory,
					Schema:      getTypeScheme(dataType),
				},
			})
		}
	}

	if headers := ans.Get("header"); headers != nil {
		for _, line := range headers.lines {
			fields := parseLineFields(line, ' ')
			returnCode := getFieldValue(fields, 0)
			paramType := getFieldValue(fields, 1)
			name := getFieldValue(fields, 2)
			comment := getFieldValue(fields, 3)
			walkOperationResponses(op, parseLineFields(returnCode, ','), func(resp *spec.Response) {
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
