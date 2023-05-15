package consul

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vizee/gapi-plus/apimeta"
	"github.com/vizee/gapi-plus/registry/consul/liteconsul"
	"github.com/vizee/gapi/metadata"
)

type fileInfo struct {
	content []*metadata.Route
	index   uint64
}

type Registry struct {
	client     *liteconsul.Client
	prefix     string
	knownIndex uint64
	files      map[string]*fileInfo
	mu         sync.Mutex
}

func (r *Registry) Sync(ctx context.Context, force bool) ([]*metadata.Route, error) {
	keyPrefix := fmt.Sprintf("%s/data/", r.prefix)

	changed := false

	r.mu.Lock()
	defer r.mu.Unlock()

	entries, rootMeta, err := r.client.KV().List(ctx, keyPrefix, nil)
	if err != nil && !liteconsul.IsNotFound(err) {
		return nil, err
	}

	if r.knownIndex == rootMeta.LastIndex && !force {
		return nil, nil
	}

	if force {
		// 直接清空已存在路由
		r.files = make(map[string]*fileInfo, len(entries))
	}

	rc := &apimeta.ResolvingCache{}

	for i := range entries {
		e := &entries[i]
		filename := e.Key[len(keyPrefix):]
		newFile := false
		fi := r.files[filename]
		if fi == nil {
			fi = &fileInfo{}
			newFile = true
		} else if fi.index == e.ModifyIndex {
			continue
		}

		sds, err := parseServiceDesc(e.Value)
		if err != nil {
			return nil, err
		}
		routes, err := apimeta.ResolveRoutes(rc, sds, false)
		if err != nil {
			return nil, err
		}

		fi.content = routes
		fi.index = e.ModifyIndex
		if newFile {
			r.files[filename] = fi
		}
		changed = true
	}

	if len(r.files) > len(entries) {
		// 把 entries 合并入 r.files 后，r.files 长度应该和 entries 相同，否则就是有多余的文件
		inuse := make(map[string]bool, len(entries))
		for _, e := range entries {
			inuse[e.Key[len(keyPrefix):]] = true
		}
		for filename := range r.files {
			if !inuse[filename] {
				delete(r.files, filename)
			}
		}
		changed = true
	}

	r.knownIndex = rootMeta.LastIndex

	if !changed {
		return nil, nil
	}

	total := 0
	for _, fi := range r.files {
		total += len(fi.content)
	}
	content := make([]*metadata.Route, 0, total)
	for _, fi := range r.files {
		content = append(content, fi.content...)
	}
	return content, nil
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

func NewRegistry(client *liteconsul.Client, prefix string) *Registry {
	return &Registry{
		client: client,
		prefix: prefix,
		files:  make(map[string]*fileInfo),
	}
}
