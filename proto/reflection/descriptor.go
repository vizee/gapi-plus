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
	files    map[string]*descriptorpb.FileDescriptorProto
	fds      []*descriptorpb.FileDescriptorProto
	visit    map[protoreflect.FullName]bool
	apiFiles map[string]bool
}

func (c *gapiServiceResolver) getFile(d protoreflect.Descriptor) *descriptorpb.FileDescriptorProto {
	f := d.ParentFile()
	fd := c.files[f.Path()]
	if fd == nil {
		fd = newFileDescriptor(f)
		c.files[f.Path()] = fd
		c.fds = append(c.fds, fd)
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

	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Kind() == protoreflect.MessageKind {
			c.resolveMessage(field.Message())
		}
	}

	fd := c.getFile(md)
	fd.MessageType = append(fd.MessageType, protodesc.ToDescriptorProto(md))

	// 只有带 route 信息的方法会调用 resolveMessage，所以相关的文件都要标记为 API File

	c.apiFiles[fd.GetName()] = true
}

func (c *gapiServiceResolver) resolveService(sd protoreflect.ServiceDescriptor) error {
	// 因为涉及到对 service 和 method 的 options 检查，所以需要手动实现 protodesc.ToServiceDescriptorProto
	if v, _ := proto.GetExtension(sd.Options(), annotation.E_Server).(string); v == "" {
		return nil
	}

	mds := sd.Methods()
	routeMethods := make([]*descriptorpb.MethodDescriptorProto, 0, mds.Len())
	for i := 0; i < mds.Len(); i++ {
		md := mds.Get(i)

		// 过滤 Streaming
		if md.IsStreamingClient() || md.IsStreamingServer() || proto.GetExtension(md.Options(), annotation.E_Http) == nil {
			continue
		}

		c.resolveMessage(md.Input())
		c.resolveMessage(md.Output())

		routeMethods = append(routeMethods, protodesc.ToMethodDescriptorProto(md))
	}

	if len(routeMethods) == 0 {
		return nil
	}

	fd := c.getFile(sd.ParentFile())
	fd.Service = append(fd.Service, &descriptorpb.ServiceDescriptorProto{
		Name:    proto.String(string(sd.Name())),
		Options: proto.Clone(sd.Options()).(*descriptorpb.ServiceOptions),
		Method:  routeMethods,
	})
	c.apiFiles[fd.GetName()] = true
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
		files:    make(map[string]*descriptorpb.FileDescriptorProto),
		visit:    make(map[protoreflect.FullName]bool),
		apiFiles: make(map[string]bool),
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

	fds := make([]*descriptorpb.FileDescriptorProto, 0, len(resolver.apiFiles))
	// files 遍历会乱序，所以要用 fds 固定顺序
	for _, fd := range resolver.fds {
		if !resolver.apiFiles[fd.GetName()] {
			continue
		}
		fds = append(fds, fd)
	}

	return &descriptorpb.FileDescriptorSet{
		File: fds,
	}, nil
}

func newFileDescriptor(f protoreflect.FileDescriptor) *descriptorpb.FileDescriptorProto {
	fd := &descriptorpb.FileDescriptorProto{
		Name: proto.String(f.Path()),
	}
	if f.Package() != "" {
		fd.Package = proto.String(string(f.Package()))
	}
	imports := f.Imports()
	for i := 0; i < imports.Len(); i++ {
		fileImport := imports.Get(i)
		fd.Dependency = append(fd.Dependency, fileImport.Path())
		if fileImport.IsPublic {
			fd.PublicDependency = append(fd.PublicDependency, int32(i))
		}
		if fileImport.IsWeak {
			fd.WeakDependency = append(fd.WeakDependency, int32(i))
		}
	}
	return fd
}
