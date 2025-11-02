package server

import (
	"github.com/OpenAudio/go-openaudio/pkg/registrar"
	"github.com/OpenAudio/go-openaudio/pkg/safemap"
)

type CorePeerProvider struct {
	// peers and signers are the same for core
	peers *safemap.SafeMap[EthAddress, registrar.Peer]
}

func NewCorePeerProvider() *CorePeerProvider {
	return &CorePeerProvider{
		peers: safemap.New[EthAddress, registrar.Peer](),
	}
}

func (p *CorePeerProvider) Peers() ([]registrar.Peer, error) {
	return p.peers.Values(), nil
}

func (p *CorePeerProvider) Signers() ([]registrar.Peer, error) {
	return p.peers.Values(), nil
}

func (s *Server) PeerProvider() registrar.PeerProvider {
	return s.peerProvider
}
