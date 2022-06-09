package sanhua

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"time"
)

var (
	bufferPool  sync.Pool
	segmentPool sync.Pool
	segmentID   uint32
	segmentSeq  uint32
)

func init() {
	bufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	segmentPool = sync.Pool{
		New: func() interface{} {
			return new(segment)
		},
	}

}

type segments struct {
	done bool

	id     uint32
	length uint32
	segIDs map[uint32]struct{}
	segs   []*segment

	body bytes.Buffer

	rAddr *net.UDPAddr
	mu    sync.Mutex
}

type segment struct {
	conn  *Conn
	rAddr *net.UDPAddr
	// header
	prot   uint8
	id     uint32
	seq    uint32
	length uint32
	ack    uint32

	// body
	body []byte

	// read time or write time
	t    time.Time
	done chan struct{}
}

type segHeader struct {
	prot   uint8
	id     uint32
	length uint32
	ack    uint32
	seq    uint32
}

type rcv struct {
	addr *net.UDPAddr
	t    time.Time
	b    []byte
}

func (s *segment) encodeHeader() []byte {
	buf := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buf)
	buf.Reset()
	binary.Write(buf, binary.BigEndian, s.prot)
	binary.Write(buf, binary.BigEndian, s.id)
	binary.Write(buf, binary.BigEndian, s.seq)
	binary.Write(buf, binary.BigEndian, s.length)
	binary.Write(buf, binary.BigEndian, s.ack)
	return buf.Bytes()
}

func (s *segment) marshal() []byte {

	buf := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buf)
	buf.Reset()
	binary.Write(buf, binary.BigEndian, s.prot)
	binary.Write(buf, binary.BigEndian, s.id)
	binary.Write(buf, binary.BigEndian, s.seq)
	binary.Write(buf, binary.BigEndian, s.length)
	binary.Write(buf, binary.BigEndian, s.ack)
	binary.Write(buf, binary.BigEndian, s.body)
	return buf.Bytes()
}

func (s *segment) unMarshal(b []byte) {

	data := bytes.NewBuffer(b)
	binary.Read(data, binary.BigEndian, &s.prot)
	binary.Read(data, binary.BigEndian, &s.id)
	binary.Read(data, binary.BigEndian, &s.seq)
	binary.Read(data, binary.BigEndian, &s.length)
	binary.Read(data, binary.BigEndian, &s.ack)
	s.body = data.Bytes()

}

func (s *segment) reset(conn *Conn, addr *net.UDPAddr) {
	s.conn = conn
	s.rAddr = addr
	s.prot = conn.prot + 1
	s.ack = 0
	s.done = make(chan struct{}, 1)

}
