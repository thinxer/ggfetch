package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"strings"
)

type HTMLFetcher struct {
	MaxItemSize int64
	Client      *http.Client
}

func (h HTMLFetcher) Generate(query neturl.Values) (content []byte, err error) {
	url := query.Get("url")
	url = escapeFragment(url)
	resp, err := h.Client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = StatusCodeError{url, resp.StatusCode}
		return
	}

	var r io.Reader = resp.Body
	if h.MaxItemSize > 0 {
		r = io.LimitReader(resp.Body, h.MaxItemSize)
	}
	buffered := bufio.NewReader(r)

	// Check Content Type
	header, _ := buffered.Peek(512)
	contentType := http.DetectContentType(header)
	if !strings.HasPrefix(contentType, "text/") {
		return
	}

	// Now read all remaining
	buf, err := ioutil.ReadAll(buffered)
	if err != nil {
		return
	}

	if newurl, escaped := escapeFragmentMeta(url, buf); escaped {
		query.Set("url", newurl)
		return h.Generate(query)
	}

	return json.Marshal(fetchResponse{unescapeFragment(resp.Request.URL.String()), buf})
}

func (h HTMLFetcher) WriteResponse(w http.ResponseWriter, cached []byte) error {
	fp := fetchResponse{}
	if err := json.Unmarshal(cached, &fp); err != nil {
		return err
	}
	w.Header().Set("X-Real-URL", fp.URL)
	_, err := w.Write(fp.Content)
	return err
}

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
