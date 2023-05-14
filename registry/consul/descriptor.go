package consul

import (
	"bytes"
	"compress/gzip"
	"io"

	"github.com/vizee/gapi-plus/proto/descriptor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func parseServiceDesc(data []byte) ([]*descriptor.ServiceDesc, error) {
	pddata, err := gzipDecompress(data)
	if err != nil {
		return nil, err
	}
	var fd descriptorpb.FileDescriptorProto
	err = proto.Unmarshal(pddata, &fd)
	if err != nil {
		return nil, err
	}
	parser := descriptor.NewParser()
	err = parser.AddFile(&fd)
	if err != nil {
		return nil, err
	}
	return parser.Services(), nil
}

func gzipDecompress(data []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	return io.ReadAll(gr)
}
