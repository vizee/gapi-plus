package annotations

import (
	"strconv"
	"strings"
	"unicode"
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

func ExtractAnnotations(comments string) Annotations {
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

func ParseLineFields(line string, sep byte) []string {
	if len(line) == 0 {
		return nil
	}
	isSpaceSep := unicode.IsSpace(rune(sep))
	i := 0
	var fields []string
	for i < len(line) {
		field, n := nextField(line[i:], sep)
		if field != "" || !isSpaceSep {
			fields = append(fields, field)
		}
		i += n
	}
	return fields
}

func GetFieldValue(fields []string, i int) string {
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
