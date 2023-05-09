package reflection

import (
	"sort"

	annotation "github.com/vizee/gapi-proto-go/gapi"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func CollectFileSet(fds *descriptorpb.FileDescriptorSet, visit map[string]bool, fd protoreflect.FileDescriptor) {
	if visit[fd.Path()] {
		return
	}

	visit[fd.Path()] = true
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(fd))

	imports := fd.Imports()
	for i := 0; i < imports.Len(); i++ {
		CollectFileSet(fds, visit, imports.Get(i).FileDescriptor)
	}
}

func CollectServerFiles(srv *grpc.Server, filter func(path string) bool) (*descriptorpb.FileDescriptorSet, error) {
	serviceInfo := srv.GetServiceInfo()
	svcNames := make([]string, 0, len(serviceInfo))
	for name := range serviceInfo {
		svcNames = append(svcNames, name)
	}
	sort.Strings(svcNames)

	fds := &descriptorpb.FileDescriptorSet{}
	visit := make(map[string]bool)
	for _, name := range svcNames {
		sd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			return nil, err
		}
		fd := sd.ParentFile()
		if !filter(fd.Path()) {
			continue
		}
		CollectFileSet(fds, visit, fd)
	}
	return fds, nil
}

type gapiServiceResolver struct {
	files map[string]*descriptorpb.FileDescriptorProto
	fdps  []*descriptorpb.FileDescriptorProto
	visit map[protoreflect.FullName]bool
}

func (c *gapiServiceResolver) getFile(d protoreflect.Descriptor) *descriptorpb.FileDescriptorProto {
	f := d.ParentFile()
	fd := c.files[f.Path()]
	if fd == nil {
		fd = &descriptorpb.FileDescriptorProto{}
		c.files[f.Path()] = fd
		c.fdps = append(c.fdps, fd)
	}
	return fd
}

func (c *gapiServiceResolver) resolveMessage(md protoreflect.MessageDescriptor) {
	msgName := md.FullName()
	if c.visit[msgName] {
		return
	}

	c.visit[msgName] = true

	// 过滤嵌套类型
	nested := md.Messages()
	for i := 0; i < nested.Len(); i++ {
		c.visit[nested.Get(i).FullName()] = true
	}

	fds := md.Fields()
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		if fd.Kind() == protoreflect.MessageKind {
			c.resolveMessage(fd.Message())
		}
	}

	f := c.getFile(md)
	f.MessageType = append(f.MessageType, protodesc.ToDescriptorProto(md))
}

func (c *gapiServiceResolver) resolveService(sd protoreflect.ServiceDescriptor) error {
	// 因为涉及到对 service 和 method 的 options 检查，所以需要手动实现 protodesc.ToServiceDescriptorProto
	if proto.GetExtension(sd.Options(), annotation.E_Server) == nil {
		return nil
	}

	mds := sd.Methods()
	methods := make([]*descriptorpb.MethodDescriptorProto, 0, mds.Len())
	for i := 0; i < mds.Len(); i++ {
		md := mds.Get(i)
		// 过滤 Streaming
		if md.IsStreamingClient() || md.IsStreamingServer() || proto.GetExtension(md.Options(), annotation.E_Http) == nil {
			continue
		}

		c.resolveMessage(md.Input())
		c.resolveMessage(md.Output())

		methods = append(methods, protodesc.ToMethodDescriptorProto(md))
	}

	f := c.getFile(sd.ParentFile())
	f.Service = append(f.Service, &descriptorpb.ServiceDescriptorProto{
		Name:    proto.String(string(sd.Name())),
		Options: proto.Clone(sd.Options()).(*descriptorpb.ServiceOptions),
		Method:  methods,
	})
	return nil
}

func CollectGapiFiles(srv *grpc.Server) (*descriptorpb.FileDescriptorSet, error) {
	serviceInfo := srv.GetServiceInfo()
	svcNames := make([]string, 0, len(serviceInfo))
	for name := range serviceInfo {
		svcNames = append(svcNames, name)
	}
	sort.Strings(svcNames)

	resolver := &gapiServiceResolver{
		files: make(map[string]*descriptorpb.FileDescriptorProto),
		visit: make(map[protoreflect.FullName]bool),
	}
	for _, name := range svcNames {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			return nil, err
		}

		err = resolver.resolveService(d.(protoreflect.ServiceDescriptor))
		if err != nil {
			return nil, err
		}
	}

	return &descriptorpb.FileDescriptorSet{
		File: resolver.fdps,
	}, nil
}
