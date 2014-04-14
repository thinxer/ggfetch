package ggfetch

import (
	"bytes"
	"hash/crc32"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/groupcache"
)

func fetch(client *http.Client, url string, maxSize int64) ([]byte, error) {
	log.Println("Fetching", url)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)

	// Check Content Type
	io.CopyN(buf, resp.Body, 512)
	contentType := http.DetectContentType(buf.Bytes())
	if !strings.HasPrefix(contentType, "text/") {
		return nil, nil
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
		return nil, err
	}
	return buf.Bytes(), nil
}

type Fetcher struct {
	group *groupcache.Group
}

func (gf Fetcher) Fetch(url string, ttl int64) ([]byte, error) {
	prefix := ":"
	if ttl > 0 {
		offset := int64(crc32.ChecksumIEEE([]byte(url))) % ttl
		id := (time.Now().Unix() + offset) / ttl
		prefix = strconv.FormatInt(id, 16) + ":"
	}
	var buf []byte
	err := gf.group.Get(nil, prefix+url, groupcache.AllocatingByteSliceSink(&buf))
	return buf, err
}

func New(name string, cacheSize int64, itemSize int64, timeout time.Duration) Fetcher {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				start := time.Now()
				conn, err := net.DialTimeout(netw, addr, timeout)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(start.Add(timeout))
				return conn, nil
			},
		},
	}
	return Fetcher{
		groupcache.NewGroup("fetch", cacheSize, groupcache.GetterFunc(
			func(_ groupcache.Context, key string, dest groupcache.Sink) error {
				url := strings.SplitN(key, ":", 2)[1]
				bytes, err := fetch(client, url, itemSize)
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
