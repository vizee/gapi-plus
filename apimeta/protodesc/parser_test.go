package protodesc

import (
	"encoding/json"
	"os"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParseRoutes(t *testing.T) {
	data, err := os.ReadFile("../../testdata/pdtest/pdtest.pd")
	if err != nil {
		t.Fatal(err)
	}

	var fds descriptorpb.FileDescriptorSet
	err = proto.Unmarshal(data, &fds)
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser()
	for _, fd := range fds.File {
		err = p.AddFile(fd, false)
		if err != nil {
			t.Fatal(err)
		}
	}
	incomplete := p.CheckIncomplete()
	if len(incomplete) > 0 {
		t.Fatal("incomplete", incomplete)
	}

	j, err := json.MarshalIndent(p.Routes(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(j))
}
