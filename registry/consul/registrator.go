package consul

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/vizee/gapi-plus/registry/consul/liteconsul"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Registrator struct {
	client *liteconsul.Client
	prefix string
}

func (r *Registrator) getChecksumKey(server string, file string) string {
	return fmt.Sprintf("%s/checksum/%s/%s", r.prefix, server, file)
}

func (r *Registrator) getDataKey(server string, file string) string {
	return fmt.Sprintf("%s/data/%s/%s", r.prefix, server, file)
}

func (r *Registrator) getChecksum(key string) (string, uint64, error) {
	entry, _, err := r.client.KV().Get(context.Background(), key, nil)
	if err != nil && !liteconsul.IsNotFound(err) {
		return "", 0, err
	}
	if entry != nil {
		return string(entry.Value), entry.ModifyIndex, nil
	}
	return "", 0, nil
}

func (r *Registrator) setFileData(chksumKey string, chksum string, dataKey string, data []byte, lastVer uint64) (bool, error) {
	ok, _, err := r.client.Txn().
		EmitKV(&liteconsul.TxnOpKV{
			Verb:  liteconsul.VerbCAS,
			Key:   chksumKey,
			Value: []byte(chksumKey),
			Index: lastVer,
		}).
		EmitKV(&liteconsul.TxnOpKV{
			Verb:  liteconsul.VerbSet,
			Key:   dataKey,
			Value: data,
		}).
		Commit()
	return ok, err
}

func (r *Registrator) syncFileData(ctx context.Context, server string, filename string, data []byte) error {
	dataKey := r.getDataKey(server, filename)
	chksumKey := r.getChecksumKey(server, filename)

	sha1sum := sha1.Sum(data)
	chksum := hex.EncodeToString(sha1sum[:])

	for ctx.Err() == nil {
		lastChksum, lastVer, err := r.getChecksum(chksumKey)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if lastChksum == chksum {
			return nil
		}

		ok, _ := r.setFileData(chksumKey, chksum, dataKey, data, lastVer)
		if ok {
			break
		}
		time.Sleep(time.Second)
	}
	return ctx.Err()
}

func (r *Registrator) notifyUpdate(ctx context.Context) error {
	key := fmt.Sprintf("%s/notify", r.prefix)
	for ctx.Err() == nil {
		_, err := r.client.KV().Put(key, []byte(strconv.Itoa(int(time.Now().UnixNano()))))
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	return ctx.Err()
}

func (r *Registrator) RegisterFiles(ctx context.Context, server string, files []*descriptorpb.FileDescriptorProto) error {
	if len(files) == 0 {
		return nil
	}

	var (
		protobuf    []byte
		compressbuf bytes.Buffer
	)
	for _, f := range files {
		var err error
		protobuf, err = proto.MarshalOptions{}.MarshalAppend(protobuf[:0], f)
		if err != nil {
			return err
		}

		compressbuf.Reset()
		err = gzipCompress(&compressbuf, protobuf)
		if err != nil {
			return err
		}
		err = r.syncFileData(ctx, server, f.GetName(), compressbuf.Bytes())
		if err != nil {
			return err
		}
	}
	return r.notifyUpdate(ctx)
}

func NewRegistrator(client *liteconsul.Client, prefix string) *Registrator {
	return &Registrator{
		client: client,
		prefix: prefix,
	}
}

func gzipCompress(buf *bytes.Buffer, data []byte) error {
	gw := gzip.NewWriter(buf)
	_, err := gw.Write(data)
	if err != nil {
		return err
	}
	return gw.Close()
}
