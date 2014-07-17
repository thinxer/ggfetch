package main

import (
	"log"
	"net/http"
	"net/url"

	"github.com/golang/groupcache"
)

type Fetcher interface {
	Generate(query url.Values) ([]byte, error)
	WriteResponse(http.ResponseWriter, []byte) error
}

// DumpContentResponse is a Fetcher mixin that will write the cached content directly to the response.
type DumpContentResponse struct{}

func (_ DumpContentResponse) WriteResponse(w http.ResponseWriter, content []byte) error {
	_, err := w.Write(content)
	return err
}

type fetcherGetter struct{ Fetcher }

func (g fetcherGetter) Get(_ groupcache.Context, key string, dest groupcache.Sink) error {
	q, err := url.ParseQuery(key)
	if err != nil {
		return err
	}
	bytes, err := g.Generate(q)
	if err != nil {
		return err
	}
	return dest.SetBytes(bytes)
}

type entry struct {
	Group *groupcache.Group
	Fetcher
}

type GGFetchHandler struct {
	methods map[string]entry
}

func (g *GGFetchHandler) Register(name string, fetcher Fetcher, size int64) {
	if g.methods == nil {
		g.methods = make(map[string]entry)
	}
	g.methods[name] = entry{
		Group:   groupcache.NewGroup(name, size, fetcherGetter{fetcher}),
		Fetcher: fetcher,
	}
}

func (g *GGFetchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Path[1:]
	key := r.URL.RawQuery
	log.Println("METHOD", method, "KEY", key)
	hi, ok := g.methods[method]
	if !ok {
		http.NotFound(w, r)
		return
	}

	var buf []byte
	if err := hi.Group.Get(nil, key, groupcache.AllocatingByteSliceSink(&buf)); err != nil {
		log.Println("ERROR", err, "METHOD", method, "KEY", key)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := hi.Fetcher.WriteResponse(w, buf); err != nil {
		log.Println("ERROR", err, "METHOD", method, "KEY", key)
	}
}
