package ggclient

import (
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// timeblock is for TTL support.
func timeblock(ttl uint32, x []byte) uint32 {
	offset := crc32.ChecksumIEEE(x) % ttl
	id := (uint32(time.Now().Unix()) + offset) / ttl
	return id
}

type Client struct {
	// hostname:port for the GGFetch service.
	Host string
	// Default TTL value for requests. It will be override explicitly in the Do method.
	TTL uint32
	// HTTP Client to use. Will use http.DefaultClient if nil.
	Client *http.Client
}

// === Raw method ===

func (c Client) Do(method string, ttl uint32, kvs ...string) (*http.Response, error) {
	if len(kvs)%2 != 0 {
		panic("key values must be in pairs")
	}

	// Choose client.
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}

	// Build up queries.
	q := url.Values{}
	for i := 0; i < len(kvs); i += 2 {
		q.Add(kvs[i], kvs[i+1])
	}
	if ttl > 0 {
		tb := timeblock(ttl, []byte(q.Encode()))
		q.Set("_t", strconv.FormatInt(int64(tb), 10))
	}
	u := url.URL{
		Scheme:   "http",
		Host:     c.Host,
		Path:     "/" + method,
		RawQuery: q.Encode(),
	}

	// Fire.
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// === Convenient methods ===

func (c Client) HTML(u string) (string, io.ReadCloser, error) {
	resp, err := c.Do("html", c.TTL, "url", u)
	if err != nil {
		return "", nil, err
	}
	return resp.Header.Get("X-Real-URL"), resp.Body, nil
}

func (c Client) Image(u string, width int) (content []byte, err error) {
	return ReadAll(c.Do("image", c.TTL, "url", u, "width", strconv.Itoa(width)))
}

func (c Client) Dimension(u string) (w, h int, err error) {
	var dim struct {
		Width, Height int
	}
	err = JSON(c.Do("dimension", c.TTL, "url", u)).Decode(&dim)
	w, h = dim.Width, dim.Height
	return
}
