/*
Changes: use query string instead of raw path for group and key.
This is due to a bug/feature in net/url, which will unescape "%2F" incorrectly.
*/

/*
Copyright 2013 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"code.google.com/p/goprotobuf/proto"
	. "github.com/golang/groupcache"
	"github.com/golang/groupcache/consistenthash"
	pb "github.com/golang/groupcache/groupcachepb"
)

// TODO: make this configurable?
const defaultBasePath = "/_groupcache/"

// TODO: make this configurable as well.
const defaultReplicas = 1

// PeersPool implements PeerPicker for a pool of HTTP peers.
type PeersPool struct {
	// Context optionally specifies a context for the server to use when it
	// receives a request.
	// If nil, the server uses a nil Context.
	Context func(*http.Request) Context

	// Transport optionally specifies an http.RoundTripper for the client
	// to use when it makes a request.
	// If nil, the client uses http.DefaultTransport.
	Transport func(Context) http.RoundTripper

	// base path including leading and trailing slash, e.g. "/_groupcache/"
	basePath string

	// this peer's base URL, e.g. "https://example.net:8000"
	self string

	mu    sync.Mutex
	peers *consistenthash.Map
}

var httpPoolMade bool

// NewPeersPool initializes an HTTP pool of peers.
// It registers itself as a PeerPicker and as an HTTP handler with the
// http.DefaultServeMux.
// The self argument be a valid base URL that points to the current server,
// for example "http://example.net:8000".
func NewPeersPool(self string) *PeersPool {
	if httpPoolMade {
		panic("groupcache: NewPeersPool must be called only once")
	}
	httpPoolMade = true
	p := &PeersPool{basePath: defaultBasePath, self: self, peers: consistenthash.New(defaultReplicas, nil)}
	RegisterPeerPicker(func() PeerPicker { return p })
	http.Handle(defaultBasePath, p)
	return p
}

// Set updates the pool's list of peers.
// Each peer value should be a valid base URL,
// for example "http://example.net:8000".
func (p *PeersPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
}

func (p *PeersPool) PickPeer(key string) (ProtoGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.peers.IsEmpty() {
		return nil, false
	}
	if peer := p.peers.Get(key); peer != p.self {
		// TODO: pre-build a slice of *httpGetter when Set()
		// is called to avoid these two allocations.
		return &httpGetter{p.Transport, peer + p.basePath}, true
	}
	return nil, false
}

func (p *PeersPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("PeersPool serving unexpected path: " + r.URL.Path)
	}

	// Parse request.
	groupName := r.FormValue("group")
	key := r.FormValue("key")

	// Fetch the value for this group/key.
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}
	var ctx Context
	if p.Context != nil {
		ctx = p.Context(r)
	}

	group.Stats.ServerRequests.Add(1)
	var value []byte
	if err := group.Get(ctx, key, AllocatingByteSliceSink(&value)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the value to the response body as a proto message.
	body, err := proto.Marshal(&pb.GetResponse{Value: value})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Write(body)
}

type httpGetter struct {
	transport func(Context) http.RoundTripper
	baseURL   string
}

func (h *httpGetter) Get(context Context, in *pb.GetRequest, out *pb.GetResponse) (err error) {
	uu, err := url.Parse(h.baseURL)
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("group", in.GetGroup())
	q.Set("key", in.GetKey())
	uu.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", uu.String(), nil)
	if err != nil {
		return err
	}
	tr := http.DefaultTransport
	if h.transport != nil {
		tr = h.transport(context)
	}
	res, err := tr.RoundTrip(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}
	// TODO: avoid this garbage.
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	err = proto.Unmarshal(b, out)
	if err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}