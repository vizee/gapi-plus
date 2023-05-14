package liteconsul

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"
)

// https://developer.hashicorp.com/consul/api-docs/kv

type KV struct {
	inner *Client
}

type KVEntry struct {
	CreateIndex uint64
	ModifyIndex uint64
	LockIndex   uint64
	Key         string
	Flags       uint64
	Value       []byte
	Session     string
}

func (kv *KV) Get(ctx context.Context, key string, options *QueryOptions) (*KVEntry, *QueryMetadata, error) {
	var ents []KVEntry
	meta, err := kv.inner.query(ctx, &request{
		method: http.MethodGet,
		path:   "/v1/kv/" + strings.TrimPrefix(key, "/"),
	}, options, &ents)
	if len(ents) > 0 {
		return &ents[0], meta, err
	}
	return nil, meta, err
}

func (kv *KV) List(ctx context.Context, prefix string, options *QueryOptions) ([]KVEntry, *QueryMetadata, error) {
	var ents []KVEntry
	meta, err := kv.inner.query(ctx, &request{
		method: http.MethodGet,
		path:   "/v1/kv/" + strings.TrimPrefix(prefix, "/"),
		params: []string{"recurse", "true"},
	}, options, &ents)
	return ents, meta, err
}

func (kv *KV) Delete(key string, params ...string) (bool, error) {
	resp, err := kv.inner.call(context.Background(), &request{
		method: http.MethodDelete,
		path:   "/v1/kv/" + strings.TrimPrefix(key, "/"),
		params: params,
	})
	if err != nil {
		return false, err
	}
	return string(resp) == "true", nil
}

func (kv *KV) Put(key string, value []byte) (bool, error) {
	return kv.put(key, value)
}

func (kv *KV) CAS(key string, value []byte, cas uint64) (bool, error) {
	return kv.put(key, value, "cas", strconv.FormatUint(cas, 10))
}

func (kv *KV) put(key string, value []byte, params ...string) (bool, error) {
	resp, err := kv.inner.call(context.Background(), &request{
		method: http.MethodPut,
		path:   "/v1/kv/" + strings.TrimPrefix(key, "/"),
		params: params,
		body:   bytes.NewReader(value),
	})
	if err != nil {
		return false, err
	}
	return string(resp) == "true", nil
}

func (c *Client) KV() *KV {
	return &KV{inner: c}
}
