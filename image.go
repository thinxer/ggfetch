package main

import (
	"bytes"
	"encoding/json"
	"hash/crc32"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/groupcache"
)

var defaultJpegOption = &jpeg.Options{Quality: 24}

type dimension struct {
	Width, Height int
}

func fetchImageDimension(client *http.Client, url string) (dimension, error) {
	resp, err := client.Get(url)
	if err != nil {
		return dimension{}, err
	}
	defer resp.Body.Close()
	c, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return dimension{}, err
	}
	return dimension{c.Width, c.Height}, nil
}

func fetchImageResize(client *http.Client, url string, width int, maxSize int64) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if maxSize > 0 {
		if s, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); s > maxSize {
			return nil, nil
		}
	}
	var r io.Reader = resp.Body
	if maxSize > 0 {
		r = io.LimitReader(resp.Body, maxSize)
	}
	im, format, err := image.Decode(r)
	switch err {
	case image.ErrFormat:
		return nil, nil
	case io.ErrUnexpectedEOF:
		return nil, nil
	default:
		return nil, err
	}

	w, h := im.Bounds().Max.X, im.Bounds().Max.Y
	if width > 0 && w > width {
		w, h = width, h*width/w
		im = Resize(im, im.Bounds(), w, h)
	}

	buf := new(bytes.Buffer)
	if format == "png" {
		err = png.Encode(buf, im)
	} else {
		err = jpeg.Encode(buf, im, defaultJpegOption)
	}
	return buf.Bytes(), err
}

type ImageFetcher struct {
	group  *groupcache.Group
	config *groupcache.Group
}

func (f ImageFetcher) key(url string, ttl int64, w int) string {
	prefix := strconv.Itoa(w) + ":"
	if ttl > 0 {
		offset := int64(crc32.ChecksumIEEE([]byte(url))) % ttl
		id := (time.Now().Unix() + offset) / ttl
		prefix += strconv.FormatInt(id, 16)
	}
	return prefix + ":" + url
}

func (f *ImageFetcher) FetchDimension(url string, ttl int64) (d dimension, err error) {
	key := f.key(url, ttl, 0)
	var buf []byte
	err = f.config.Get(nil, key, groupcache.AllocatingByteSliceSink(&buf))
	if err != nil {
		return
	}
	err = json.NewDecoder(bytes.NewReader(buf)).Decode(&d)
	return
}

func (f ImageFetcher) Fetch(url string, ttl int64, w int) (im []byte, err error) {
	key := f.key(url, ttl, w)
	err = f.group.Get(nil, key, groupcache.AllocatingByteSliceSink(&im))
	return
}

func (f ImageFetcher) CacheStats(which groupcache.CacheType) groupcache.CacheStats {
	return f.group.CacheStats(which)
}

func NewImageFetcher(name string, cacheSize int64, itemSize int64, client *http.Client) ImageFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	var getter, configGetter groupcache.GetterFunc
	getter = func(context groupcache.Context, key string, dest groupcache.Sink) error {
		parts := strings.SplitN(key, ":", 3)
		url := parts[2]
		width, _ := strconv.Atoi(parts[0])
		response, err := fetchImageResize(client, url, width, itemSize)
		if err != nil {
			return err
		}
		dest.SetBytes(response)
		return nil
	}
	configGetter = func(context groupcache.Context, key string, dest groupcache.Sink) error {
		log.Println("Fetching image: ", key)

		url := strings.SplitN(key, ":", 3)[2]
		c, err := fetchImageDimension(client, url)
		if err != nil {
			return err
		}
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(c); err != nil {
			return err
		}
		dest.SetBytes(buf.Bytes())
		return nil
	}
	return ImageFetcher{
		groupcache.NewGroup(name+".main", cacheSize, getter),
		// TODO 8192 is just a randomly chosen ratio...
		groupcache.NewGroup(name+".config", cacheSize/8192, configGetter),
	}
}
