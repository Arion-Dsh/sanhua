/*
Package sanhua provides a mini version of TCP top on UDP, but with out resend
lost packet.

*/
package sanhua

import (
	"errors"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	max_readbytes = 2 << 9 //1024 byte
	max_sequence  = (1 << 32) - 1
)

// Listen announces on the local network address.
// network must be "udp", "udp4" or "udp6"
// more [net.ListenPacket](https://pkg.go.dev/net#ListenConfig.ListenPacket)
func Listen(network, address string) (*Conn, error) {

	err := checkNewwork(network)
	if err != nil {
		return nil, err
	}

	addr, err := net.ResolveUDPAddr(network, address)

	if err != nil {
		return nil, err
	}

	return ListenUDP(network, addr)

}

// ListenUDP like Listen but for existed *net.UDPaddr
func ListenUDP(network string, addr *net.UDPAddr) (*Conn, error) {

	err := checkNewwork(network)
	if err != nil {
		return nil, err
	}

	c, err := net.ListenUDP(network, addr)

	if err != nil {
		return nil, err
	}

	l := NewConn(c, addr, nil)

	return l, nil
}

// Dial connects to the address on the named network.
// udp's local address is automatically chosen
// network must be "udp", "udp4", "udp6".
// more [net.Dial](https://pkg.go.dev/net#Dial)
func Dial(network, address string) (*Conn, error) {

	err := checkNewwork(network)
	if err != nil {
		return nil, err
	}

	laddr, err := net.ResolveUDPAddr(network, ":0")
	if err != nil {
		return nil, err
	}

	raddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}

	return DialUDP(network, laddr, raddr)
}

// DialUDP connects with existed UDP address.
// laddr and raddr not be nil.
// see Dial.
func DialUDP(network string, lAddr, rAddr *net.UDPAddr) (*Conn, error) {

	err := checkNewwork(network)
	if err != nil {
		return nil, err
	}

	c, err := net.ListenUDP(network, lAddr)

	if err != nil {
		return nil, err
	}
	l := NewConn(c, lAddr, rAddr)

	return l, nil
}

/* // Conn send and receive data packets upon a network stream connection. */
type Conn struct {
	seq uint32

	udp   *net.UDPConn
	lAddr *net.UDPAddr
	rAddr *net.UDPAddr

	writeQ map[uint32]*segment

	rcvSeg   map[uint32]*segment
	rcvSegs  map[uint32]*segments
	rcvEvent chan struct{}

	readQ chan *segments

	rcv chan *rcv

	rcvPacket chan *Packet
	ackChache *ackChache

	rtt time.Duration

	r    int
	prot uint8
}

func NewConn(udp *net.UDPConn, l, r *net.UDPAddr) *Conn {
	c := &Conn{
		udp:   udp,
		lAddr: l,
		rAddr: r,

		writeQ:   map[uint32]*segment{},
		rcvSeg:   map[uint32]*segment{},
		rcvSegs:  map[uint32]*segments{},
		rcvEvent: make(chan struct{}),
		readQ:    make(chan *segments),

		rcv:       make(chan *rcv),
		rcvPacket: make(chan *Packet),

		r:    max_readbytes,
		prot: 42,
	}

	c.ackChache = newAckCache()
	c.rcvWatch()
	return c
}

func (c *Conn) ReadFromUDP(p []byte) (int, *net.UDPAddr, error) {

	s := <-c.readQ
	n, err := s.body.Read(p)
	return n, s.rAddr, err
}

func (c *Conn) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {

	if addr == nil {
		return 0, nil
	}

	id := atomic.AddUint32(&segmentID, 1)
	s := [][]byte{}
	for {
		w := make([]byte, c.r)
		n := copy(w, b)

		s = append(s, w[:n])

		if n < c.r {
			break
		}
		b = b[n:]
	}

	var wg sync.WaitGroup
	for _, buf := range s {
		wg.Add(1)

		go func(id uint32, buf []byte) {
			defer wg.Done()
			seg := segmentPool.Get().(*segment)
			defer segmentPool.Put(seg)

			seg.reset(c, addr)
			seg.id = id
			seg.seq = atomic.AddUint32(&segmentSeq, 1)
			seg.length = uint32(len(s))
			seg.body = buf

			p := seg.marshal()
			c.udp.WriteTo(p, addr)
			seg.t = time.Now()

			c.writeQ[seg.seq] = seg

			<-seg.done
		}(id, buf)

	}
	wg.Wait()

	return len(b), nil
}

func (c *Conn) rcvWatch() {

	go func() {
		for {
			rcv := <-c.rcv
			switch rcv.b[0] {
			case c.prot:
				pkt := new(Packet)
				err := pkt.unMarshal(rcv.b)
				if err != nil {
					continue
				}
				pkt.addr = rcv.addr
				c.rcvPacket <- pkt

			case c.prot + 1:
				c.reverseSeg(rcv)
			case c.prot + 2:
				c.checkSegRcv(rcv)
			}

		}
	}()

	go func() {

		for {
			<-c.rcvEvent
			for k, v := range c.rcvSeg {
				s, ok := c.rcvSegs[v.id]
				if !ok {
					s = &segments{
						id:     v.id,
						length: v.length,
						segIDs: map[uint32]struct{}{},
						segs:   []*segment{},
						rAddr:  v.rAddr,
					}

				}
				c.rcvSegs[v.id] = s
				s.mu.Lock()
				if _, ok := s.segIDs[k]; !ok {
					s.segIDs[k] = struct{}{}
					s.segs = append(s.segs, v)
				}
				s.mu.Unlock()
				delete(c.rcvSeg, k)
				if int(s.length) == len(s.segs) {
					sort.Slice(s.segs, func(i, j int) bool {
						return s.segs[i].seq < s.segs[j].seq
					})

					for i := range s.segs {
						s.body.Write(s.segs[i].body)
					}
					s.done = true
					c.readQ <- s

				}
			}
		}

	}()

	go func() {
		for {
			buf := make([]byte, c.r)
			n, addr, err := c.udp.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			switch buf[0] {
			case c.prot, c.prot + 1, c.prot + 2:
			default:
				continue
			}
			b := make([]byte, n)
			copy(b, buf)
			c.rcv <- &rcv{t: time.Now(), addr: addr, b: b[:n]}
		}
	}()
}

func (c *Conn) reverseSeg(rcv *rcv) {
	seg := new(segment)
	seg.unMarshal(rcv.b)
	seg.prot += 2
	seg.ack = seg.seq
	seg.rAddr = rcv.addr
	c.rcvSeg[seg.seq] = seg
	p := seg.encodeHeader()
	c.udp.WriteTo(p, rcv.addr)
	c.rcvEvent <- struct{}{}
}

func (c *Conn) checkSegRcv(rcv *rcv) {

	seg := segmentPool.Get().(*segment)
	defer segmentPool.Put(seg)
	seg.reset(c, nil)
	seg.unMarshal(rcv.b)

	rcvSeg, ok := c.writeQ[seg.ack]

	if ok {
		delete(c.writeQ, seg.ack)
		rcvSeg.done <- struct{}{}
	}

}

func (c *Conn) PacketReadFrom() (*Packet, *net.UDPAddr, error) {
	pkt := <-c.rcvPacket
	return pkt, pkt.addr, nil
}

func (c *Conn) PacketWriteTo(pkt *Packet, addr *net.UDPAddr) error {

	pkt.prot = c.prot

	pkt.ack = pkt.sequence
	pkt.sequence = atomic.AddUint32(&c.seq, 1)

	c.ackChache.cache(addr.String(), pkt)
	data, _ := pkt.marshal()

	_, err := c.udp.WriteToUDP(data, addr)
	if err != nil {
		return err
	}
	return nil
}

func (c *Conn) LocalAddr() net.Addr {
	return c.udp.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.udp.RemoteAddr()
}

// Sequence is local sequence for udp send
// each time send a packet increase the local sequence number
func (c *Conn) Sequence() uint32 {
	return c.seq
}

// SetDeadline sets the read and write deadlines associated with the endpoint.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.udp.SetDeadline(t)
}

// SetReadDeadline implements *net.UDPConn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.udp.SetReadDeadline(t)
}

// SetWriteDeadline implements *net.UDPConn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.udp.SetWriteDeadline(t)
}

func (c *Conn) Close() error {
	return c.udp.Close()
}

// SetReadBuffer implements *net.UDPConn SetReadBuffer method.
func (c *Conn) SetReadBuffer(bytes int) error {
	c.r = bytes * 2
	return c.udp.SetReadBuffer(bytes)
}

func checkNewwork(n string) error {
	switch n {
	case "udp", "udp4", "udp6":
		return nil
	}
	return errors.New("sanhua: error nework must be \"udp\" \"udp4\" or \"udp6\" .")
}

func sequenceGreaterThan(s1, s2 uint32) bool {
	return ((s1 > s2) && (s1-s2 <= max_sequence/2)) ||
		((s1 < s2) && (s2-s1 > max_sequence/2))
}
