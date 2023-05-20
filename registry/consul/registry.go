package consul

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/vizee/gapi-plus/registry/consul/liteconsul"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ServerFileManager interface {
	UpdateServer(server string, updated []*descriptorpb.FileDescriptorProto, deleted []string) error
	RemoveServer(server string) error
}

type serverInfo struct {
	files map[string]uint64
}

type Registry struct {
	prefix     string
	client     *liteconsul.Client
	manager    ServerFileManager
	knownIndex uint64
	servers    map[string]*serverInfo
	mu         sync.Mutex
}

func (r *Registry) syncServerFiles(ctx context.Context, prefix string, server string, entries []liteconsul.KVEntry) error {
	si := r.servers[server]
	if si == nil {
		si = &serverInfo{
			files: make(map[string]uint64),
		}
		r.servers[server] = si
	}

	updated := make([]*descriptorpb.FileDescriptorProto, 0, 3)
	for i := range entries {
		e := &entries[i]
		fname := e.Key[len(prefix)+len(server)+1:]
		modIndex, found := si.files[fname]
		if found && modIndex == e.ModifyIndex {
			continue
		}

		fd, err := parseFileDescriptor(e.Value)
		if err != nil {
			return err
		}

		updated = append(updated, fd)

		si.files[fname] = e.ModifyIndex
	}

	var deleted []string
	if len(si.files) > len(entries) {
		inuse := make(map[string]bool, len(entries))
		for i := range entries {
			e := &entries[i]
			fname := e.Key[len(prefix)+len(server)+1:]
			inuse[fname] = true
		}
		deleted = make([]string, 0, len(si.files)-len(entries))
		for name := range si.files {
			if !inuse[name] {
				delete(si.files, name)
				deleted = append(deleted, name)
			}
		}
	}

	return r.manager.UpdateServer(server, updated, deleted)
}

func (r *Registry) Sync(ctx context.Context, force bool) error {
	keyPrefix := fmt.Sprintf("%s/data/", r.prefix)

	r.mu.Lock()
	defer r.mu.Unlock()

	entries, rootMeta, err := r.client.KV().List(ctx, keyPrefix, nil)
	if err != nil && !liteconsul.IsNotFound(err) {
		return err
	}

	if r.knownIndex == rootMeta.LastIndex && !force {
		return nil
	}
	// 不论成功或者失败都不再处理这个 index
	r.knownIndex = rootMeta.LastIndex

	if force {
		// 强制更新时直接清空已有数据
		r.servers = make(map[string]*serverInfo)
	}

	serving := make(map[string]bool)
	var group string
	start := 0
	for i := range entries {
		e := &entries[i]
		fname := e.Key[len(keyPrefix):]
		slash := strings.IndexByte(fname, '/')
		if slash < 0 {
			return fmt.Errorf("invalid data key: %s", e.Key)
		}
		serverName := fname[:slash]
		if i == 0 {
			group = serverName
			serving[group] = true
		} else if group != serverName {
			err := r.syncServerFiles(ctx, keyPrefix, group, entries[start:i])
			if err != nil {
				return err
			}
			group = serverName
			serving[group] = true
			start = i
		}
	}
	if len(entries) > 0 {
		err := r.syncServerFiles(ctx, keyPrefix, group, entries[start:])
		if err != nil {
			return err
		}
	}

	if len(r.servers) != len(serving) {
		for name := range r.servers {
			if !serving[name] {
				delete(r.servers, name)

				err := r.manager.RemoveServer(name)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *Registry) Watch(ctx context.Context, update func() error) error {
	key := fmt.Sprintf("%s/notify", r.prefix)
	lastNotified := uint64(0)
	for ctx.Err() == nil {
		_, meta, err := r.client.KV().Get(ctx, key, &liteconsul.QueryOptions{
			LastIndex: lastNotified,
			WaitTime:  60 * time.Second,
		})
		if err != nil || lastNotified == meta.LastIndex {
			if liteconsul.IsNotFound(err) {
				lastNotified = meta.LastIndex
			}
			time.Sleep(time.Second)
			continue
		}
		err = update()
		if err != nil {
			return nil
		}
		lastNotified = meta.LastIndex
	}
	return nil
}

func NewRegistry(client *liteconsul.Client, prefix string, manager ServerFileManager) *Registry {
	return &Registry{
		prefix:     strings.TrimPrefix(prefix, "/"),
		client:     client,
		manager:    manager,
		knownIndex: 0,
	}
}

func gzipDecompress(data []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	return io.ReadAll(gr)
}

func parseFileDescriptor(data []byte) (*descriptorpb.FileDescriptorProto, error) {
	pddata, err := gzipDecompress(data)
	if err != nil {
		return nil, err
	}
	fd := &descriptorpb.FileDescriptorProto{}
	err = proto.Unmarshal(pddata, fd)
	if err != nil {
		return nil, err
	}
	return fd, nil
}
