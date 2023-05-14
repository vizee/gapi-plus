package liteconsul

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type QueryOptions struct {
	LastIndex uint64
	WaitTime  time.Duration
}

type QueryMetadata struct {
	LastIndex uint64
}

type request struct {
	method string
	path   string
	params []string
	body   io.Reader
}

type Client struct {
	addr  string
	token string
}

func (c *Client) send(ctx context.Context, r *request) (*http.Response, error) {
	u := strings.Builder{}
	n := len(c.addr) + len(r.path)
	for i := 0; i+1 < len(r.params); i += 2 {
		n = 1 + len(r.params[i]) + 1 + len(r.params[i+1])
	}
	u.Grow(n)
	u.WriteString(c.addr)
	u.WriteString(r.path)
	for i := 0; i+1 < len(r.params); i += 2 {
		if i == 0 {
			u.WriteByte('&')
		} else {
			u.WriteByte('?')
		}
		u.WriteString(url.QueryEscape(r.params[i]))
		u.WriteByte('=')
		u.WriteString(url.QueryEscape(r.params[i+1]))
	}

	req, err := http.NewRequestWithContext(ctx, r.method, u.String(), r.body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Consul-Token", c.token)
	}
	return http.DefaultClient.Do(req)
}

func (c *Client) call(ctx context.Context, req *request) ([]byte, error) {
	resp, err := c.send(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(resp)
	}
	return readBody(resp), nil
}

func (c *Client) query(ctx context.Context, req *request, o *QueryOptions, out interface{}) (*QueryMetadata, error) {
	if o != nil {
		if o.LastIndex > 0 {
			req.params = append(req.params, "index", strconv.FormatUint(o.LastIndex, 10))
		}
		if o.WaitTime > 0 {
			req.params = append(req.params, "wait", strconv.FormatInt(o.WaitTime.Milliseconds(), 10)+"ms")
		}
	}
	resp, err := c.send(ctx, req)
	if err != nil {
		return nil, err
	}

	qm := &QueryMetadata{}
	qm.LastIndex, _ = strconv.ParseUint(resp.Header.Get("X-Consul-Index"), 10, 64)

	if resp.StatusCode == http.StatusOK {
		if out != nil {
			err = json.Unmarshal(readBody(resp), out)
		} else {
			discardBody(resp)
		}
	} else {
		err = httpError(resp)
	}

	return qm, err
}

func NewClient(addr string, token string) *Client {
	addr = strings.TrimSuffix(addr, "/")
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	return &Client{
		addr:  addr,
		token: token,
	}
}

func readBody(resp *http.Response) []byte {
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return data
}

func discardBody(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func httpError(resp *http.Response) error {
	return &Error{Code: resp.StatusCode, Content: string(readBody(resp))}
}
