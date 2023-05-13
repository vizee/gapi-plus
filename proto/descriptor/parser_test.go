package descriptor

import (
	"encoding/json"
	"os"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParseProtoDescriptor(t *testing.T) {
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
		err := p.AddFile(fd)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, md := range p.msgs {
		if md.Incomplete {
			t.Fatal("message '" + md.Name + "' is incomplete")
		}
	}
	sds := p.Services()
	j, err := json.MarshalIndent(sds, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(j))
}
