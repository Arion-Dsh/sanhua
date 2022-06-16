package sanhua

import (
	"bytes"
	"net"
	"sync"
	"time"
)

var (
	bufferPool sync.Pool
	rcvPool    sync.Pool
)

func init() {
	bufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}

	rcvPool = sync.Pool{
		New: func() interface{} {
			return &rcv{
				pktIDs: map[uint32]struct{}{},
			}

		},
	}
}

type rcv struct {
	addr *net.UDPAddr
	t    time.Time
	b    []byte

	id     uint32
	size   int
	pktIDs map[uint32]struct{}
	pkts   []*Packet
	done   bool

	body bytes.Buffer

	mux sync.Mutex
}

func (r *rcv) reset() {
	r.b = []byte{}
	r.id = 0
	r.size = 0
	r.pktIDs = map[uint32]struct{}{}
	r.pkts = []*Packet{}
	r.done = false
}
