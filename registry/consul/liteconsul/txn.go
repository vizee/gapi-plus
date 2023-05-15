package liteconsul

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// https://developer.hashicorp.com/consul/api-docs/txn

const (
	VerbSet = "set"
	VerbCAS = "cas"
)

type TxnError struct {
	OpIndex int    `json:"OpIndex"`
	What    string `json:"What"`
}

type TxnResponse struct {
	Results []map[string]json.RawMessage `json:"Results"`
	Errors  []TxnError                   `json:"Errors"`
}

type TxnOpKV struct {
	Verb    string
	Key     string
	Value   []byte `json:",omitempty"`
	Flags   uint64 `json:",omitempty"`
	Index   uint64 `json:",omitempty"`
	Session string `json:",omitempty"`
}

type Txn struct {
	c    *Client
	buf  bytes.Buffer
	more bool
}

func (txn *Txn) emitOp(op string, entity any) {
	if txn.more {
		txn.buf.WriteByte(',')
	} else {
		txn.more = true
	}
	txn.buf.WriteString(`{"`)
	txn.buf.WriteString(op)
	txn.buf.WriteString(`":`)
	data, _ := json.Marshal(entity)
	txn.buf.Write(data)
	txn.buf.WriteByte('}')
}

func (txn *Txn) EmitKV(kv *TxnOpKV) *Txn {
	txn.emitOp("KV", kv)
	return txn
}

func (txn *Txn) Commit() (bool, *TxnResponse, error) {
	txn.buf.WriteByte(']')
	resp, err := txn.c.send(context.Background(), &request{
		method: http.MethodPut,
		path:   "/v1/txn",
		body:   &txn.buf,
	})
	if err != nil {
		return false, nil, err
	}
	if resp.StatusCode == 200 || resp.StatusCode == 409 {
		body := readBody(resp)
		var txnResp TxnResponse
		err = json.Unmarshal(body, &txnResp)
		if err != nil {
			return false, nil, err
		}
		return resp.StatusCode == 200, &txnResp, nil
	}
	return false, nil, httpError(resp)
}

func (c *Client) Txn() *Txn {
	tx := &Txn{
		c: c,
	}
	tx.buf.WriteByte('[')
	return tx
}
