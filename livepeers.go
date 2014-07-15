package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

const peerTimeout = 5 * time.Second

type peer struct {
	Peer     string
	LastSeen time.Time
}

type PeersManager struct {
	mu    sync.Mutex
	peers []peer
}

func (p *PeersManager) Get() (ret []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := 0; i < len(p.peers); i++ {
		if time.Now().Sub(p.peers[i].LastSeen) < peerTimeout {
			ret = append(ret, p.peers[i].Peer)
		}
	}
	return
}

func (p *PeersManager) Ping(pp string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := 0; i < len(p.peers); i++ {
		if p.peers[i].Peer == pp {
			p.peers[i].LastSeen = time.Now()
			return
		}
	}
	p.peers = append(p.peers, peer{
		Peer:     pp,
		LastSeen: time.Now(),
	})
}

func (p *PeersManager) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if peer := request.FormValue("peer"); peer != "" {
		p.Ping("http://" + peer)
	}
	json.NewEncoder(response).Encode(p.Get())
}

func (p *PeersManager) Heartbeat(url string, setpeers func(peers ...string)) {
	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Println("!!! ERROR Cannot connect to master:", err)
			time.Sleep(time.Second)
			continue
		}
		req.Close = true
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println("!!! ERROR Cannot connect to master:", err)
			time.Sleep(time.Second)
			continue
		}
		var livePeers []string
		if err := json.NewDecoder(resp.Body).Decode(&livePeers); err != nil {
			// Will this happen?
			panic(err)
		}
		resp.Body.Close()
		setpeers(livePeers...)

		time.Sleep(3 * time.Second)
	}
}
