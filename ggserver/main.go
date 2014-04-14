package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/thinxer/ggfetch"

	"github.com/golang/groupcache"
	"gopkg.in/yaml.v1"
)

type Config struct {
	Listen      string   // API Listen Address
	Me          string   // as ip:port
	Peers       []string // as a list of ip:port
	CacheSize   int64    `yaml:"cache_size"`    // in MB
	MaxItemSize int64    `yaml:"max_item_size"` // in KB
}

var (
	flagConfigFile = flag.String("config", "config.yml", "Config file to use.")
)

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
		panic(http.ListenAndServe(config.Me, http.HandlerFunc(peers.ServeHTTP)))
	}()

	// Setup GGFetch
	fetcher := ggfetch.New("fetch", config.CacheSize<<20, config.MaxItemSize<<10, 30*time.Second)

	// Setup
	http.HandleFunc("/fetch", func(response http.ResponseWriter, request *http.Request) {
		request.ParseForm()
		url := request.FormValue("url")
		ttl_s := request.FormValue("ttl")
		ttl, err := strconv.ParseInt(ttl_s, 10, 64)
		if err != nil {
			ttl = 0
		}
		buf, err := fetcher.Fetch(url, ttl)
		if err != nil {
			log.Println("Error while fetching:", err)
		}
		_, err = response.Write(buf)
		if err != nil {
			log.Println("Error while writing response:", err)
		}
	})
	log.Println("Listening on", config.Listen)
	panic(http.ListenAndServe(config.Listen, nil))
}
