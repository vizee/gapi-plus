package extractor

import (
	"github.com/vizee/gapi-plus/apimeta"
	"github.com/vizee/gapi-plus/proto/descriptor"
	"github.com/vizee/gapi/metadata"
	"google.golang.org/protobuf/types/descriptorpb"
)

type RouteExtractor struct {
}

func (RouteExtractor) Extract(fds []*descriptorpb.FileDescriptorProto) ([]*metadata.Route, error) {
	parser := descriptor.NewParser()
	for _, fd := range fds {
		err := parser.AddFile(fd)
		if err != nil {
			return nil, err
		}
	}
	return apimeta.ResolveRoutes(&apimeta.ResolvingCache{}, parser.Services(), false)
}
