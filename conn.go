/*
Package sanhua provides a mini version of TCP top on UDP, but with out resend
lost packet.

*/
package sanhua

import (
	"errors"
	"math"
	"net"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	max_sequence = (1 << 32) - 1
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
	proto byte
	gid   uint32
	seq   uint32

	udp   *net.UDPConn
	lAddr *net.UDPAddr
	rAddr *net.UDPAddr

	writeQ sync.Map
	qMux   sync.RWMutex
	rcvs   sync.Map
	rcvMux sync.RWMutex
	readQ  chan *rcv

	rcv       chan *rcv
	rcvPacket chan *Packet
	acks      *ackChache

	rtt  time.Duration
	rttC uint8

	readDeadline  time.Time
	writeDeadline time.Time

	mtu int // 576- 60(IP header) - 8 (UDP header) - 21 (data header) = 487

}

func NewConn(udp *net.UDPConn, l, r *net.UDPAddr) *Conn {
	c := &Conn{
		proto: 42,

		udp:   udp,
		lAddr: l,
		rAddr: r,

		readQ: make(chan *rcv),

		rcv:       make(chan *rcv),
		rcvPacket: make(chan *Packet),

		rtt: 42 * time.Millisecond,
		mtu: 487,
	}

	c.acks = newAckCache()
	c.rcvWatch()
	return c
}

// ReadFromUDP read data from the connection, copy payload to p
func (c *Conn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return c.ReadFromUDP(p)
}

// WriteTo writes a packet with payload p to addr.
func (c *Conn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	a, ok := addr.(*net.UDPAddr)
	if !ok {
		return 0, errors.New("sanhua: must be net.UDPAddr")
	}

	return c.WriteToUDP(p, a)
}

// ReadFromUDP read data from the connection, copy payload to p
func (c *Conn) ReadFromUDP(p []byte) (int, *net.UDPAddr, error) {

	s := <-c.readQ
	n, err := s.body.Read(p)
	return n, s.addr, err
}

// WriteToUDP writes packet to addr. and allow len(p) > MTU
// and p will split to N Packets
func (c *Conn) WriteToUDP(p []byte, addr *net.UDPAddr) (int, error) {

	now := time.Now()
	if addr == nil {
		return 0, nil
	}

	s := [][]byte{}
	for {
		w := make([]byte, c.mtu)
		n := copy(w, p)

		s = append(s, w[:n])

		if n < c.mtu {
			break
		}
		p = p[n:]
	}

	var wg sync.WaitGroup

	errs := make(chan error)
	gid := atomic.AddUint32(&c.gid, 1)
	size := len(s)
	for _, buf := range s {
		wg.Add(1)

		seq := atomic.AddUint32(&c.seq, 1)
		go func(gid, seq uint32, buf []byte) {
			defer wg.Done()

			pk := NewPacket()

			pk.t = now
			pk.proto = c.proto + 1
			pk.sequence = seq

			pk.body.Write(u32Bytes(gid))
			pk.body.Write(u32Bytes(uint32(size)))
			pk.body.Write(buf)

			p, _ := pk.marshal()

			_, err := c.udp.WriteTo(p, addr)
			if err != nil {
				errs <- err
				return
			}
			c.writeQ.Store(pk.sequence, pk)

			ticker := time.NewTicker(c.rtt)
			for {
				select {
				case <-time.After(1 * time.Second):
					errs <- os.ErrDeadlineExceeded
					return
				case <-pk.done:
					return
				case <-ticker.C:
					_, err := c.udp.WriteTo(p, addr)
					if err != nil {
						errs <- err
						return
					}
				}
			}
		}(gid, seq, buf)

	}
	wg.Wait()
	close(errs)

	if len(errs) != 0 {
		err := <-errs
		return 0, err
	}

	return len(p), nil
}

func (c *Conn) rcvWatch() {

	go func() {
		for {
			rcv := <-c.rcv
			switch rcv.b[0] {
			case c.proto:
				pkt := new(Packet)
				err := pkt.unMarshal(rcv.b)
				if err != nil {
					continue
				}
				pkt.addr = rcv.addr
				c.rcvPacket <- pkt

			case c.proto + 1:
				go c.composeRcv(rcv)
			case c.proto + 2:
				go c.checkRcv(rcv)
			}

		}
	}()

	go func() {
		for {
			buf := make([]byte, c.mtu)
			n, addr, err := c.udp.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			switch buf[0] {
			case c.proto, c.proto + 1, c.proto + 2:
				c.rcv <- &rcv{t: time.Now(), addr: addr, b: buf[:n]}
			}
		}
	}()
}

func (c *Conn) composeRcv(r *rcv) {
	pkt := NewPacket()
	pkt.unMarshal(r.b)
	pkt.proto += 1
	pkt.ack = pkt.sequence
	pkt.addr = r.addr

	field, ok := c.acks.cache(r.addr.String(), pkt.ack)
	pkt.ackField = field

	header, err := pkt.marshalHeader()

	if err != nil {
		return
	}

	c.udp.WriteTo(header, r.addr)

	if ok {
		return
	}

	p := make([]byte, 4)
	pkt.body.Read(p)
	gid := bytesU32(p)
	pkt.body.Read(p)
	size := bytesU32(p)
	rs := &rcv{
		id:     gid,
		size:   int(size),
		addr:   r.addr,
		pktIDs: map[uint32]struct{}{},
	}

	rsi, _ := c.rcvs.LoadOrStore(rs.id, rs)
	rs, _ = rsi.(*rcv)

	rs.mux.Lock()
	if _, ok := rs.pktIDs[pkt.ack]; !ok {
		rs.pktIDs[pkt.ack] = struct{}{}
		rs.pkts = append(rs.pkts, pkt)
	}
	if rs.size == len(rs.pkts) {
		sort.Slice(rs.pkts, func(i, j int) bool {
			return rs.pkts[j].ack > rs.pkts[i].ack
		})

		for i := range rs.pkts {
			rs.pkts[i].body.WriteTo(&rs.body)
		}

		c.rcvs.Delete(rs.id)

		c.readQ <- rs
	}
	rs.mux.Unlock()

}

func (c *Conn) checkRcv(rcv *rcv) {

	pkt := packetPool.Get().(*Packet)
	defer packetPool.Put(pkt)
	pkt.Reset()
	pkt.unMarshal(rcv.b)
	pkt.addr = rcv.addr

	c.removePkt(pkt.ack)

	acks := pkt.AckField()
	for i := range acks {
		c.removePkt(acks[i])
	}

}
func (c *Conn) removePkt(ack uint32) {
	pi, ok := c.writeQ.LoadAndDelete(ack)
	if ok {
		p := pi.(*Packet)
		p.done <- struct{}{}
		now := time.Now()
		go c.changeRTT(p.t, now)

	}
}

// changeRTT we change rtt to bad rtt immediately, drop 10 times good rtt.
// Because the arrival of packets varies with network jitter,
//we need to smooth this value to provide something meaningful,
// 10% seems prefect.
func (c *Conn) changeRTT(n, nn time.Time) {

	f := float64(nn.Sub(n)) * 1.1

	d := time.Duration(int64(math.Ceil(f)))

	if c.rtt < d {
		c.rtt = d
		c.rttC = 0
	}
	if c.rtt > d {
		if c.rttC > 10 {
			c.rtt = d
			c.rttC = 0
		} else {
			c.rttC += 1
		}
	}
}

func (c *Conn) PacketReadFrom() (*Packet, *net.UDPAddr, error) {
	pkt := <-c.rcvPacket
	return pkt, pkt.addr, nil
}

func (c *Conn) PacketWriteTo(pkt *Packet, addr *net.UDPAddr) error {

	pkt.proto = c.proto

	pkt.ack = pkt.sequence
	pkt.sequence = atomic.AddUint32(&c.seq, 1)

	c.acks.cache(addr.String(), pkt.ack)
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
	c.writeDeadline = t
	c.readDeadline = t
	return c.udp.SetDeadline(t)
}

// SetReadDeadline implements *net.UDPConn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return c.udp.SetReadDeadline(t)
}

// SetWriteDeadline implements *net.UDPConn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.readDeadline = t
	return c.udp.SetWriteDeadline(t)
}

func (c *Conn) Close() error {
	return c.udp.Close()
}

func (c *Conn) RTT() time.Duration {
	return c.rtt
}

// SetReadBuffer implements *net.UDPConn SetReadBuffer method.
func (c *Conn) SetReadBuffer(bytes int) error {
	c.mtu = bytes
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
