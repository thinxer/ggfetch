package main

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"strconv"

	_ "golang.org/x/image/bmp"
)

var defaultJpegOption = &jpeg.Options{Quality: 80}

type ImageFetcher struct {
	MaxItemSize int64
	Client      *http.Client

	DumpContentResponse
}

func (i ImageFetcher) Generate(q neturl.Values) (content []byte, err error) {
	url := q.Get("url")
	log.Println("GET URL", url)
	width, _ := strconv.Atoi(q.Get("width"))
	height, _ := strconv.Atoi(q.Get("height"))

	resp, err := i.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if i.MaxItemSize > 0 {
		if s, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); s > i.MaxItemSize {
			return nil, errors.New("image too large")
		}
	}

	var r io.Reader = resp.Body
	if i.MaxItemSize > 0 {
		r = io.LimitReader(resp.Body, i.MaxItemSize)
	}
	im, format, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	w, h := im.Bounds().Max.X, im.Bounds().Max.Y
	needToResize := false
	if width > 0 && w > width {
		w, h = width, h*width/w
		needToResize = true
	}
	if height > 0 && h > height {
		w, h = w*height/h, height
		needToResize = true
	}
	if needToResize {
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
