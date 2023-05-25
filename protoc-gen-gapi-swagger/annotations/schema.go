package annotations

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/go-openapi/spec"
)

var builtinSchemas = map[string]func() *spec.Schema{
	"boolean":   spec.BoolProperty,
	"string":    spec.StringProperty,
	"double":    spec.Float64Property,
	"float":     spec.Float32Property,
	"int8":      spec.Int8Property,
	"int16":     spec.Int16Property,
	"int32":     spec.Int32Property,
	"int64":     spec.Int64Property,
	"date":      spec.DateProperty,
	"date-time": spec.DateTimeProperty,
}

func newPrimitiveSchema(ty string) *spec.Schema {
	schema := builtinSchemas[ty]
	if schema != nil {
		return schema()
	}
	return &spec.Schema{SchemaProps: spec.SchemaProps{Type: []string{ty}}}
}

type parser struct {
	s string
	i int
}

func (p *parser) skipspace() {
	i := p.i
	for i < len(p.s) {
		if !unicode.IsSpace(rune(p.s[i])) {
			break
		}
		i++
	}
	p.i = i
}

func (p *parser) parseIdent() string {
	b := p.i
	i := p.i
	for i < len(p.s) {
		c := p.s[i]
		if !(('0' <= c && c <= '9') ||
			('A' <= c && c <= 'Z') ||
			('a' <= c && c <= 'z') ||
			c == '.' || c == '_') {
			break
		}
		i++
	}
	p.i = i
	return p.s[b:i]
}

func (p *parser) parseComposedSchema(lead *spec.Schema) *spec.Schema {
	props := spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:       []string{"object"},
			Properties: make(spec.SchemaProperties),
		},
	}
	p.i++ // skip '{'
	for p.i < len(p.s) {
		p.skipspace()
		if p.i >= len(p.s) {
			break
		}
		if p.s[p.i] == '}' {
			p.i++
			break
		}
		if p.s[p.i] == ',' {
			p.i++
			continue
		}

		override := p.parseIdent()
		p.skipspace()
		if p.i >= len(p.s) {
			break
		}
		if p.s[p.i] != '=' && p.s[p.i] != ':' {
			break
		}
		p.i++
		p.skipspace()
		item := p.parseSchema()
		if item == nil {
			break
		}
		props.Properties[override] = *item
	}
	return spec.ComposedSchema(*lead, props)
}

func (p *parser) parseSchema() *spec.Schema {
	p.skipspace()
	switch {
	case strings.HasPrefix(p.s[p.i:], "[]"):
		p.i += 2
		return spec.ArrayProperty(p.parseSchema())
	case strings.HasPrefix(p.s[p.i:], "map["):
		p.i += 4
		bracket := strings.IndexByte(p.s[p.i:], ']')
		if bracket < 0 {
			return nil
		}
		p.i += bracket + 1
		return spec.MapProperty(p.parseSchema())
	}

	var schema *spec.Schema
	id := p.parseIdent()
	switch {
	case id == "":
		return nil
	case id == "any":
		schema = newPrimitiveSchema("object")
	case builtinSchemas[id] != nil:
		schema = newPrimitiveSchema(id)
	default:
		schema = spec.RefSchema(fmt.Sprintf("#/definitions/%s", id))
	}

	if strings.HasPrefix(p.s[p.i:], "{") {
		schema = p.parseComposedSchema(schema)
	}
	return schema
}

func parseObjectSchema(dataType string) *spec.Schema {
	return (&parser{s: dataType}).parseSchema()
}

func parseAPISchema(paramType string, dataType string) *spec.Schema {
	paramType = strings.TrimPrefix(strings.TrimSuffix(paramType, "}"), "{")

	switch paramType {
	case "object":
		return parseObjectSchema(dataType)
	case "array":
		return spec.ArrayProperty(parseObjectSchema(dataType))
	default:
		return &spec.Schema{SchemaProps: spec.SchemaProps{Type: []string{paramType}, Format: dataType}}
	}
}
