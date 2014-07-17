package ggclient

import (
	"hash/crc32"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func timeblock(ttl uint32, x []byte) uint32 {
	offset := crc32.ChecksumIEEE(x) % ttl
	id := (uint32(time.Now().Unix()) + offset) / ttl
	return id
}

type Client struct {
	Host   string
	Client *http.Client
}

func (c *Client) Do(method string, ttl uint32, kvs ...string) (*http.Response, error) {
	if len(kvs)%2 != 0 {
		panic("key values must be in pairs")
	}
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
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}
