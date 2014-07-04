package main

import (
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
