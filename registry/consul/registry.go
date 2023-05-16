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

type Extractor[T any] interface {
	Extract(fds []*descriptorpb.FileDescriptorProto) (T, error)
}

type fileInfo struct {
	fd    *descriptorpb.FileDescriptorProto
	index uint64
}

type serverInfo[T any] struct {
	files   map[string]*fileInfo
	content T
	dirty   bool
}

type Registry[T any, E Extractor[T]] struct {
	prefix     string
	client     *liteconsul.Client
	extractor  E
	knownIndex uint64
	servers    map[string]*serverInfo[T]
	mu         sync.Mutex
}

func (r *Registry[T, E]) syncServerFiles(ctx context.Context, prefix string, server string, entries []liteconsul.KVEntry) (bool, error) {
	si := r.servers[server]
	if si == nil {
		si = &serverInfo[T]{
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
			return false, err
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
		content, err := r.extractor.Extract(fds)
		if err != nil {
			return false, err
		}
		si.content = content
		si.dirty = false
	}

	return dirty, nil
}

func (r *Registry[T, E]) Sync(ctx context.Context, force bool) ([]T, error) {
	keyPrefix := fmt.Sprintf("%s/data/", r.prefix)

	r.mu.Lock()
	defer r.mu.Unlock()

	entries, rootMeta, err := r.client.KV().List(ctx, keyPrefix, nil)
	if err != nil && !liteconsul.IsNotFound(err) {
		return nil, err
	}

	if r.knownIndex == rootMeta.LastIndex && !force {
		return nil, nil
	}
	// 不论成功或者失败都不再处理这个 index
	r.knownIndex = rootMeta.LastIndex

	if force {
		// 强制更新时直接清空已有数据
		r.servers = make(map[string]*serverInfo[T])
	}

	mergeContent := false
	var group string
	start := 0
	for i := range entries {
		e := &entries[i]
		fname := e.Key[len(keyPrefix):]
		slash := strings.IndexByte(fname, '/')
		if slash < 0 {
			return nil, fmt.Errorf("invalid data key: %s", e.Key)
		}
		serverName := fname[:slash]
		if i == 0 {
			group = serverName
		} else if group != serverName {
			updated, err := r.syncServerFiles(ctx, keyPrefix, group, entries[start:i])
			if err != nil {
				return nil, err
			}
			if updated {
				mergeContent = true
			}
			group = serverName
			start = i
		}
	}
	if len(entries) > 0 {
		changed, err := r.syncServerFiles(ctx, keyPrefix, group, entries[start:])
		if err != nil {
			return nil, err
		}
		if changed {
			mergeContent = true
		}
	}

	if !mergeContent {
		return nil, nil
	}

	content := make([]T, 0, len(r.servers))
	for _, si := range r.servers {
		content = append(content, si.content)
	}
	return content, nil
}

func (r *Registry[T, E]) Watch(ctx context.Context, update func() error) error {
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

func NewRegistry[T any, E Extractor[T]](client *liteconsul.Client, prefix string, extractor E) *Registry[T, E] {
	return &Registry[T, E]{
		prefix:     prefix,
		client:     client,
		extractor:  extractor,
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
