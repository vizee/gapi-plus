package gen

import (
	"reflect"
	"testing"
)

func Test_extractAnnotations(t *testing.T) {
	ans := extractAnnotations(`GetUserInfo 获取用户信息
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

func Test_parseLineFields(t *testing.T) {
	type args struct {
		line string
		sep  byte
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{name: "simple", args: args{line: "user, getter", sep: ','}, want: []string{"user", "getter"}},
		{name: "by_space", args: args{line: "  a  b            \"c \"  de  f\" g\"  ", sep: ' '}, want: []string{"a", "b", "\"c \"", "de", "f\" g\""}},
		{name: "by_comma", args: args{line: "  a, b ,,  ,   ,  \"c,\" ,de ,f\",g\"  ", sep: ','}, want: []string{"a", "b", "", "", "", "\"c,\"", "de", "f\",g\""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLineFields(tt.args.line, tt.args.sep); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLineFields() = %q, want %q", got, tt.want)
			}
		})
	}
}
