package apimeta

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/vizee/gapi-plus/proto/descriptor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestResolveRoutes(t *testing.T) {
	data, err := os.ReadFile("../testdata/pdtest/pdtest.pd")
	if err != nil {
		t.Fatal(err)
	}

	var fds descriptorpb.FileDescriptorSet
	err = proto.Unmarshal(data, &fds)
	if err != nil {
		t.Fatal(err)
	}
	p := descriptor.NewParser()
	for _, fd := range fds.File {
		err := p.AddFile(fd)
		if err != nil {
			t.Fatal(err)
		}
	}
	routes, err := ResolveRoutes(&ResolvingCache{}, p.Services(), false)
	if err != nil {
		t.Fatal(err)
	}
	j, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(j))
}
