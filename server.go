package main

import (
	"bytes"
	"flag"
	"hash/crc32"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v1"

	"github.com/golang/groupcache"
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

func fetch(url string, maxSize int64) ([]byte, error) {
	log.Println("Fetching", url)
	resp, err := http.Get(url)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)

	// Check Content Type
	io.CopyN(buf, resp.Body, 512)
	contentType := http.DetectContentType(buf.Bytes())
	if !strings.HasPrefix(contentType, "text/") {
		return nil, nil
	}

	// Copy remaining content.
	if maxSize == 0 {
		_, err = io.Copy(buf, resp.Body)
	} else {
		remaining := maxSize - 512
		if remaining > 0 {
			_, err = io.CopyN(buf, resp.Body, remaining)
		}
	}
	if err != io.EOF {
		return nil, err
	}
	return buf.Bytes(), nil
}

func main() {
	flag.Parse()

	bytes, err := ioutil.ReadFile(*flagConfigFile)
	if err != nil {
		panic(err)
	}
	config := Config{}
	yaml.Unmarshal(bytes, &config)

	// Setup groupcache
	peers := groupcache.NewHTTPPool("http://" + config.Me)
	peersList := []string{}
	for _, peer := range config.Peers {
		peersList = append(peersList, "http://"+peer)
	}
	peers.Set(peersList...)
	group := groupcache.NewGroup("fetch", config.CacheSize*(1<<20), groupcache.GetterFunc(
		func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
			url := strings.SplitN(key, ":", 2)[1]
			bytes, err := fetch(url, config.MaxItemSize*1024)
			if err != nil {
				return err
			}
			dest.SetBytes(bytes)
			return nil
		}))
	go func() {
		panic(http.ListenAndServe(config.Me, http.HandlerFunc(peers.ServeHTTP)))
	}()

	// Setup
	http.HandleFunc("/fetch", func(response http.ResponseWriter, request *http.Request) {
		request.ParseForm()
		url := request.FormValue("url")
		ttl_s := request.FormValue("ttl")
		ttl, err := strconv.ParseInt(ttl_s, 10, 64)
		if err != nil {
			ttl = 0
		}
		prefix := ":"
		if ttl > 0 {
			offset := int64(crc32.ChecksumIEEE([]byte(url))) % ttl
			id := (time.Now().Unix() + offset) / ttl
			prefix = strconv.FormatInt(id, 16) + ":"
		}
		var buf []byte
		err = group.Get(nil, prefix+url, groupcache.AllocatingByteSliceSink(&buf))
		if err != nil {
			log.Println("Error while group.Get:", err)
		}
		_, err = response.Write(buf)
		if err != nil {
			log.Println("Error while writing response:", err)
		}
	})
	log.Println("Listening on", config.Listen)
	panic(http.ListenAndServe(config.Listen, nil))
}
