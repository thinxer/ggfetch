package ggfetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/groupcache"
)

type StatusCodeError struct {
	URL  string
	Code int
}

func (r StatusCodeError) Error() string {
	return fmt.Sprintf("Response code %d for URL: %s", r.Code, r.URL)
}

type fetchResponse struct {
	URL     string
	Content []byte
}

func fetch(client *http.Client, url string, maxSize int64) (response fetchResponse, err error) {
	log.Println("Fetching", url)
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = StatusCodeError{url, resp.StatusCode}
		return
	}

	buf := new(bytes.Buffer)
	// Check Content Type
	io.CopyN(buf, resp.Body, 512)
	contentType := http.DetectContentType(buf.Bytes())
	if !strings.HasPrefix(contentType, "text/") {
		return
	}

	// Copy remaining content.
	if maxSize == 0 {
		_, err = io.Copy(buf, resp.Body)
	} else {
		remaining := maxSize - 512
		if remaining > 0 {
			_, err = io.CopyN(buf, resp.Body, remaining)
		}
	}
	if err != io.EOF {
		return
	}
	return fetchResponse{URL: resp.Request.URL.String(), Content: buf.Bytes()}, nil
}

type Fetcher struct {
	group *groupcache.Group
}

func (gf Fetcher) Fetch(url string, ttl int64) (realUrl string, content []byte, err error) {
	prefix := ":"
	if ttl > 0 {
		offset := int64(crc32.ChecksumIEEE([]byte(url))) % ttl
		id := (time.Now().Unix() + offset) / ttl
		prefix = strconv.FormatInt(id, 16) + ":"
	}
	var buf []byte
	err = gf.group.Get(nil, prefix+url, groupcache.AllocatingByteSliceSink(&buf))
	if err != nil {
		return
	}
	var response fetchResponse
	err = json.Unmarshal(buf, &response)
	if err != nil {
		return
	}
	return response.URL, response.Content, nil
}

func New(name string, cacheSize int64, itemSize int64, client *http.Client) Fetcher {
	if client == nil {
		client = http.DefaultClient
	}
	return Fetcher{
		groupcache.NewGroup("fetch", cacheSize, groupcache.GetterFunc(
			func(_ groupcache.Context, key string, dest groupcache.Sink) error {
				url := strings.SplitN(key, ":", 2)[1]
				response, err := fetch(client, url, itemSize)
				if err != nil {
					return err
				}
				bytes, err := json.Marshal(response)
				if err != nil {
					return err
				}
				dest.SetBytes(bytes)
				return nil
			})),
	}
}

func Get(name string) Fetcher {
	return Fetcher{groupcache.GetGroup(name)}
}
