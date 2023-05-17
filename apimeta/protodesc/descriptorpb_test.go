package protodesc

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/vizee/gapi/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestResolveRoutes(t *testing.T) {
	data, err := os.ReadFile("../../testdata/pdtest/pdtest.pd")
	if err != nil {
		t.Fatal(err)
	}

	var fds descriptorpb.FileDescriptorSet
	err = proto.Unmarshal(data, &fds)
	if err != nil {
		t.Fatal(err)
	}
	var routes []*metadata.Route
	p := NewParser()
	for _, fd := range fds.File {
		routes, err = p.AddFile(routes, fd, false)
		if err != nil {
			t.Fatal(err)
		}
	}

	j, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(j))
}
