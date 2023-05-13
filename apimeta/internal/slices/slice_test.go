package slices

import (
	"bytes"
	"testing"
)

func TestMerge(t *testing.T) {
	type args struct {
		a []byte
		b []byte
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{name: "empty"},
		{name: "left", args: args{a: []byte("hello")}, want: []byte("hello")},
		{name: "right", args: args{b: []byte("world")}, want: []byte("world")},
		{name: "join", args: args{a: []byte("hello"), b: []byte("world")}, want: []byte("helloworld")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Merge(tt.args.a, tt.args.b); !bytes.Equal(got, tt.want) {
				t.Errorf("Merge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShrink(t *testing.T) {
	type args struct {
		a []int
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "empty"},
		{name: "equal", args: args{a: make([]int, 2, 2)}},
		{name: "shrink", args: args{a: make([]int, 1, 2)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Shrink(tt.args.a); len(got) != cap(got) {
				t.Errorf("Shrink() = %v, want %v", cap(got), len(got))
			}
		})
	}
}
