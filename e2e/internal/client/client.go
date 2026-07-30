// Package client is a thin HTTP wrapper so test cases read as intent
// ("GET /interaction/ping with this token") instead of net/http boilerplate.
package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL  string
	http     *http.Client
	token    string
	sourceIP string
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
	}
}

// WithToken returns a copy of the client that sends the given bearer token.
// The original is unchanged, so tests can share one base client safely.
func (c *Client) WithToken(token string) *Client {
	cp := *c
	cp.token = token
	return &cp
}

// WithSourceIP returns a copy that presents the given client IP via
// X-Forwarded-For. Giving each test a unique IP isolates the gateway's per-IP
// rate-limit bucket, so heavy tests (rate limit, load balancing) don't starve
// each other.
func (c *Client) WithSourceIP(ip string) *Client {
	cp := *c
	cp.sourceIP = ip
	return &cp
}

// Response is a decoded HTTP response convenient for assertions.
type Response struct {
	Status  int
	Headers http.Header
	Body    []byte
}

// JSON unmarshals the body into v.
func (r *Response) JSON(v any) error { return json.Unmarshal(r.Body, v) }

// Do issues a request, optionally with a JSON body and extra headers.
func (c *Client) Do(method, path string, body any, headers map[string]string) (*Response, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.sourceIP != "" {
		req.Header.Set("X-Forwarded-For", c.sourceIP)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Headers: resp.Header, Body: data}, nil
}

// Convenience wrappers.
func (c *Client) Get(path string) (*Response, error) {
	return c.Do(http.MethodGet, path, nil, nil)
}
func (c *Client) Post(path string, body any) (*Response, error) {
	return c.Do(http.MethodPost, path, body, nil)
}
func (c *Client) Put(path string, body any) (*Response, error) {
	return c.Do(http.MethodPut, path, body, nil)
}
func (c *Client) Delete(path string) (*Response, error) {
	return c.Do(http.MethodDelete, path, nil, nil)
}
