package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"runtime"
	"time"

	"code.google.com/p/go.net/publicsuffix"
	"github.com/facebookgo/grace/gracehttp"
	"github.com/golang/groupcache"
	"gopkg.in/yaml.v1"
)

type Config struct {
	HTML struct {
		CacheSize   int64 `yaml:"cache_size"`
		MaxItemSize int64 `yaml:"max_item_size"`
	}
	Image struct {
		CacheSize   int64 `yaml:"cache_size"`
		MaxItemSize int64 `yaml:"max_item_size"`
	}
	Dimension struct {
		CacheSize int64 `yaml:"cache_size"`
	}
}

var (
	flagConfigFile  = flag.String("config", "ggfetch.yml", "Config file to use.")
	flagBind        = flag.String("bind", "localhost", "Address to bind on. Special value ec2 will use the local ipv4 address and localhost instead.")
	flagPort        = flag.Int("port", 9001, "Port to listen on.")
	flagListenLocal = flag.Bool("listenlocal", false, "Listen to 127.0.0.1 in addition to the bind address.")
	flagMaster      = flag.String("master", "", "Master server to get config from.")
)

var (
	defaultHTTPClient *http.Client
)

// http client
func init() {
	timeout := 30 * time.Second
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	check(err)
	defaultHTTPClient = &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		Jar:       jar,
		Timeout:   timeout,
	}
}

func main() {
	flag.Parse()

	// Special case for ec2 binding
	if *flagBind == "ec2" {
		resp, err := http.Get("http://169.254.169.254/latest/meta-data/local-ipv4/")
		if err != nil {
			panic(err)
		}
		content, err := ioutil.ReadAll(resp.Body)
		check(err)
		resp.Body.Close()
		*flagBind = string(content)
		*flagListenLocal = true
	}

	me := fmt.Sprintf("%s:%d", *flagBind, *flagPort)
	var config Config

	if *flagMaster == "" {
		bytes, err := ioutil.ReadFile(*flagConfigFile)
		check(err)
		yaml.Unmarshal(bytes, &config)
		*flagMaster = me
	} else {
		log.Println("Getting config from master:", *flagMaster)
		resp, err := http.Get(fmt.Sprintf("http://%s/config", *flagMaster))
		check(err)
		check(json.NewDecoder(resp.Body).Decode(&config))
		resp.Body.Close()
	}
	log.Printf("Config loaded: %#v", config)

	// Setup GGFetch
	ggfetch := new(GGFetchHandler)
	ggfetch.Register("html", HTMLFetcher{
		MaxItemSize: config.HTML.MaxItemSize << 10,
		Client:      defaultHTTPClient,
	}, config.HTML.CacheSize<<20)
	ggfetch.Register("image", ImageFetcher{
		MaxItemSize: config.Image.MaxItemSize << 10,
		Client:      defaultHTTPClient,
	}, config.Image.CacheSize<<20)
	ggfetch.Register("dimension", DimensionFetcher{
		Client: defaultHTTPClient,
	}, config.Dimension.CacheSize<<20)

	// Fetchers
	http.Handle("/", ggfetch)

	http.HandleFunc("/config", func(response http.ResponseWriter, request *http.Request) {
		json.NewEncoder(response).Encode(config)
	})

	http.HandleFunc("/stats", func(response http.ResponseWriter, request *http.Request) {
		var stats struct {
			Goroutines int
			Caches     map[string]groupcache.CacheStats
		}
		stats.Goroutines = runtime.NumGoroutine()
		stats.Caches = make(map[string]groupcache.CacheStats)
		for name, handler := range ggfetch.methods {
			stats.Caches[name] = handler.Group.CacheStats(groupcache.MainCache)
			stats.Caches[name+"_hot"] = handler.Group.CacheStats(groupcache.HotCache)
		}
		json.NewEncoder(response).Encode(stats)
	})

	// Peers
	peers := NewPeersPool("http://" + me)
	peersManager := new(PeersManager)
	http.Handle("/ping", peersManager)
	go peersManager.Heartbeat(fmt.Sprintf("http://%s/ping?peer=%s", *flagMaster, me), peers.Set)

	var servers []*http.Server
	servers = append(servers, &http.Server{
		Addr:         me,
		Handler:      nil,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
	})
	if *flagListenLocal {
		servers = append(servers, &http.Server{
			Addr:         fmt.Sprintf("localhost:%d", *flagPort),
			Handler:      nil,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 60 * time.Second,
		})
	}
	check(gracehttp.Serve(servers...))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
