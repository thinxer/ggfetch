package main

import (
	"encoding/json"
	"image"
	"net/http"
	"net/url"
)

type dimension struct {
	Width, Height int
}

type DimensionFetcher struct {
	Client *http.Client

	DumpContentResponse
}

func (d DimensionFetcher) Generate(query url.Values) (content []byte, err error) {
	u := query.Get("url")
	resp, err := d.Client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return nil, err
	}
	return json.Marshal(dimension{c.Width, c.Height})
}
