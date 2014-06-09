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
	Listen      string   // API Listen Address
	Me          string   // as ip:port
	Peers       []string // as a list of ip:port
	CacheSize   int64    `yaml:"cache_size"`    // in MB
	MaxItemSize int64    `yaml:"max_item_size"` // in KB
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
	fetcher := New("fetch", config.CacheSize<<20, config.MaxItemSize<<10, defaultHTTPClient)

	// Setup
	http.HandleFunc("/fetch", func(response http.ResponseWriter, request *http.Request) {
		request.ParseForm()
		url := request.FormValue("url")
		ttl_s := request.FormValue("ttl")
		ttl, err := strconv.ParseInt(ttl_s, 10, 64)
		if err != nil {
			ttl = 0
		}
		var buf []byte
		url, buf, err = fetcher.Fetch(url, ttl)
		if err != nil {
			log.Println("Error while fetching:", err)
		}
		response.Header().Set("X-Real-URL", url)
		_, err = response.Write(buf)
		if err != nil {
			log.Println("Error while writing response:", err)
		}
	})

	http.HandleFunc("/stats", func(response http.ResponseWriter, request *http.Request) {
		json.NewEncoder(response).Encode(struct {
			Main, Hot groupcache.CacheStats
		}{fetcher.CacheStats(groupcache.MainCache), fetcher.CacheStats(groupcache.HotCache)})
	})

	log.Println("Listening on", config.Listen)
	panic(http.ListenAndServe(config.Listen, nil))
}
