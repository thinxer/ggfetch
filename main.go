package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"

	"code.google.com/p/go.net/publicsuffix"
	"github.com/golang/groupcache"
	"gopkg.in/yaml.v1"
)

type Config struct {
	Listen           string   // API Listen Address
	Me               string   // as ip:port
	Peers            []string // as a list of ip:port
	CacheSize        int64    `yaml:"cache_size"`          // in MB
	MaxItemSize      int64    `yaml:"max_item_size"`       // in KB
	ImageCacheSize   int64    `yaml:"image_cache_size"`    // in MB
	ImageMaxItemSize int64    `yaml:"image_max_item_size"` // in KB
}

var (
	flagConfigFile = flag.String("config", "config.yml", "Config file to use.")
)

var (
	defaultHTTPClient *http.Client
)

// http client
func init() {
	timeout := 30 * time.Second
	timeoutDialer := func(netw, addr string) (net.Conn, error) {
		start := time.Now()
		conn, err := net.DialTimeout(netw, addr, timeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(start.Add(timeout))
		return conn, nil
	}
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		panic(err)
	}
	defaultHTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial:  timeoutDialer,
		},
		Jar: jar,
	}
}

func main() {
	flag.Parse()

	bytes, err := ioutil.ReadFile(*flagConfigFile)
	if err != nil {
		panic(err)
	}
	config := Config{}
	yaml.Unmarshal(bytes, &config)

	// Setup groupcache peers
	peers := groupcache.NewHTTPPool("http://" + config.Me)
	peersList := []string{}
	for _, peer := range config.Peers {
		peersList = append(peersList, "http://"+peer)
	}
	peers.Set(peersList...)
	go func() {
		panic(http.ListenAndServe(config.Me, peers))
	}()

	// Setup GGFetch
	htmlFetcher := NewHTMLFetcher("fetch", config.CacheSize<<20, config.MaxItemSize<<10, defaultHTTPClient)
	imageFetcher := NewImageFetcher("image", config.ImageCacheSize<<20, config.ImageMaxItemSize<<10, defaultHTTPClient)

	// Setup
	http.HandleFunc("/fetch", func(response http.ResponseWriter, request *http.Request) {
		url := request.FormValue("url")
		ttl, _ := strconv.ParseInt(request.FormValue("ttl"), 10, 64)
		log.Println("Fetching HTML:", url)

		realUrl, buf, err := htmlFetcher.Fetch(url, ttl)
		if err != nil {
			log.Println("Error while fetching HTML:", err)
		}
		response.Header().Set("X-Real-URL", realUrl)
		_, err = response.Write(buf)
		if err != nil {
			log.Println("Error while writing response:", err)
		}
	})

	http.HandleFunc("/resize", func(response http.ResponseWriter, request *http.Request) {
		url := request.FormValue("url")
		ttl, _ := strconv.ParseInt(request.FormValue("ttl"), 10, 64)
		width, _ := strconv.Atoi(request.FormValue("width"))
		log.Println("Fetching image:", url, width)

		bytes, err := imageFetcher.Fetch(url, ttl, width)
		if err != nil {
			log.Println("Error while fetching image:", err)
		}
		_, err = response.Write(bytes)
		if err != nil {
			log.Println("Error while writing response:", err)
		}
	})

	http.HandleFunc("/dimension", func(response http.ResponseWriter, request *http.Request) {
		url := request.FormValue("url")
		log.Println("Fetching dimension:", url)

		ttl, _ := strconv.ParseInt(request.FormValue("ttl"), 10, 64)

		config, err := imageFetcher.FetchDimension(url, ttl)
		if err != nil {
			log.Println("Error while fetching image config:", err)
		}
		err = json.NewEncoder(response).Encode(config)
		if err != nil {
			log.Println("Error while writing response:", err)
		}
	})

	http.HandleFunc("/stats", func(response http.ResponseWriter, request *http.Request) {
		var stats struct {
			HTML, Image struct {
				Main, Hot groupcache.CacheStats
			}
		}
		stats.HTML.Main = htmlFetcher.CacheStats(groupcache.MainCache)
		stats.HTML.Hot = htmlFetcher.CacheStats(groupcache.HotCache)
		stats.Image.Main = htmlFetcher.CacheStats(groupcache.MainCache)
		stats.Image.Hot = htmlFetcher.CacheStats(groupcache.HotCache)
		json.NewEncoder(response).Encode(stats)
	})

	log.Println("Listening on", config.Listen)
	server := &http.Server{
		Addr:         config.Listen,
		Handler:      nil,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	panic(server.ListenAndServe())
}