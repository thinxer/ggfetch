package ggclient

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

func ReadAll(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

type Decoder interface {
	Decode(v interface{}) error
}

// JSON can be used to decode a single value.
func JSON(resp *http.Response, err error) Decoder {
	if err != nil {
		return jsonHelper{err: err}
	}
	return jsonHelper{rc: resp.Body}
}

type jsonHelper struct {
	err error
	rc  io.ReadCloser
}

func (j jsonHelper) Decode(v interface{}) error {
	if j.err != nil {
		return j.err
	}
	defer j.rc.Close()
	return json.NewDecoder(j.rc).Decode(v)
}
