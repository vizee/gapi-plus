package annotations

import (
	"reflect"
	"testing"
)

func TestExtractAnnotations(t *testing.T) {
	ans := ExtractAnnotations(`GetUserInfo 获取用户信息
@summary 获取用户信息
@description 根据 {uid} 获取用户基本信息
@tags user
@tags getter
@consume json
@produce json
@deprecated
`)
	for _, a := range ans {
		t.Log(a.name, "=", a.LineNum(), "{", a.Line(-1), "}")
	}
}

func TestParseLineFields(t *testing.T) {
	type args struct {
		line string
		sep  byte
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{name: "empty", args: args{line: "", sep: ','}, want: []string{}},
		{name: "space", args: args{line: "       ", sep: ' '}, want: []string{}},
		{name: "simple", args: args{line: "user, getter", sep: ','}, want: []string{"user", "getter"}},
		{name: "by_space", args: args{line: "  a  b            \"c \"  de  f\" g\"  ", sep: ' '}, want: []string{"a", "b", "\"c \"", "de", "f\" g\""}},
		{name: "by_comma", args: args{line: "  a, b ,,  ,   ,  \"c,\" ,de ,f\",g\"  ", sep: ','}, want: []string{"a", "b", "", "", "", "\"c,\"", "de", "f\",g\""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLineFields(tt.args.line, tt.args.sep); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLineFields() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetFieldValue(t *testing.T) {
	type args struct {
		fields []string
		i      int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "empty", args: args{fields: []string{}, i: 0}, want: ""},
		{name: "simpe", args: args{fields: []string{"abc"}, i: 0}, want: "abc"},
		{name: "quoted", args: args{fields: []string{"\"abc\""}, i: 0}, want: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetFieldValue(tt.args.fields, tt.args.i); got != tt.want {
				t.Errorf("GetFieldValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
