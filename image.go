package main

import (
	"bytes"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	neturl "net/url"
	"strconv"
)

var defaultJpegOption = &jpeg.Options{Quality: 24}

type ImageFetcher struct {
	MaxItemSize int64
	Client      *http.Client

	DumpContentResponse
}

func (i ImageFetcher) Generate(q neturl.Values) (content []byte, err error) {
	url := q.Get("url")
	width, _ := strconv.Atoi(q.Get("width"))

	resp, err := i.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if i.MaxItemSize > 0 {
		if s, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); s > i.MaxItemSize {
			return nil, nil
		}
	}
	var r io.Reader = resp.Body
	if i.MaxItemSize > 0 {
		r = io.LimitReader(resp.Body, i.MaxItemSize)
	}
	im, format, err := image.Decode(r)
	switch err {
	case nil:
		break
	case image.ErrFormat, io.ErrUnexpectedEOF:
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
