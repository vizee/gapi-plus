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

type Updater interface {
	Update(server string, fds []*descriptorpb.FileDescriptorProto) error
}

type fileInfo struct {
	fd    *descriptorpb.FileDescriptorProto
	index uint64
}

type serverInfo struct {
	files map[string]*fileInfo
	dirty bool
}

type Registry[U Updater] struct {
	prefix     string
	client     *liteconsul.Client
	updater    U
	knownIndex uint64
	servers    map[string]*serverInfo
	mu         sync.Mutex
}

func (r *Registry[U]) syncServerFiles(ctx context.Context, prefix string, server string, entries []liteconsul.KVEntry) error {
	si := r.servers[server]
	if si == nil {
		si = &serverInfo{
			files: make(map[string]*fileInfo),
		}
		r.servers[server] = si
	}

	dirty := false
	for i := range entries {
		e := &entries[i]
		fname := e.Key[len(prefix)+len(server)+1:]
		newFile := false
		fi := si.files[fname]
		if fi == nil {
			fi = &fileInfo{}
			newFile = true
		} else if fi.index == e.ModifyIndex {
			continue
		}

		fd, err := parseFileDescriptor(e.Value)
		if err != nil {
			return err
		}

		fi.fd = fd
		fi.index = e.ModifyIndex

		if newFile {
			si.files[fname] = fi
		}
		dirty = true
	}

	if len(si.files) > len(entries) {
		inuse := make(map[string]bool, len(entries))
		for i := range entries {
			e := &entries[i]
			fname := e.Key[len(prefix)+len(server)+1:]
			inuse[fname] = true
		}
		for name := range si.files {
			if !inuse[name] {
				delete(si.files, name)
			}
		}
		dirty = true
	}

	if dirty {
		si.dirty = dirty
	}

	if si.dirty {
		fds := make([]*descriptorpb.FileDescriptorProto, 0, len(si.files))
		for _, fi := range si.files {
			fds = append(fds, fi.fd)
		}
		err := r.updater.Update(server, fds)
		if err != nil {
			return err
		}
		si.dirty = false
	}

	return nil
}

func (r *Registry[U]) Sync(ctx context.Context, force bool) error {
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

	servers := make(map[string]bool)
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
			servers[group] = true
		} else if group != serverName {
			err := r.syncServerFiles(ctx, keyPrefix, group, entries[start:i])
			if err != nil {
				return err
			}
			group = serverName
			servers[group] = true
			start = i
		}
	}
	if len(entries) > 0 {
		err := r.syncServerFiles(ctx, keyPrefix, group, entries[start:])
		if err != nil {
			return err
		}
	}

	if len(r.servers) != len(servers) {
		for name := range r.servers {
			if !servers[name] {
				delete(r.servers, name)
				err := r.updater.Update(name, nil)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *Registry[U]) Watch(ctx context.Context, update func() error) error {
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

func NewRegistry[U Updater](client *liteconsul.Client, prefix string, updater U) *Registry[U] {
	return &Registry[U]{
		prefix:     strings.TrimPrefix(prefix, "/"),
		client:     client,
		updater:    updater,
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
