/*
Package sanhua provides a mini version of TCP top on UDP, but with out resend
lost packet.

*/
package sanhua

import (
	"errors"
	"net"
	"sync"
	"time"
)

const (
	max_readbytes = 2 << 10 //2046b
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

	addr, err := net.ResolveUDPAddr("udp", address)

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

	l := NewConn(c)

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
func DialUDP(network string, laddr, raddr *net.UDPAddr) (*Conn, error) {

	err := checkNewwork(network)
	if err != nil {
		return nil, err
	}

	c, err := net.DialUDP(network, laddr, raddr)

	if err != nil {
		return nil, err
	}
	l := NewConn(c)

	return l, nil
}

// Conn send and receive data packets upon a network stream connection.
type Conn struct {
	udp *net.UDPConn

	ackChache *ackChache

	lSequence uint32
	mux       sync.Mutex

	pool sync.Pool

	r int // max read bytes
}

//NewConn new connection with existed upd connection.
func NewConn(udp *net.UDPConn) *Conn {
	l := &Conn{udp: udp, r: max_readbytes}
	l.ackChache = newAckCache()
	l.pool = sync.Pool{
		New: func() interface{} {
			return NewPacket()
		},
	}
	return l

}

func (c *Conn) LocalAddr() net.Addr {
	return c.udp.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.udp.RemoteAddr()
}

func (c *Conn) ReadFrom() (*Packet, *net.UDPAddr, error) {

	buf := make([]byte, c.r)
	n, addr, err := c.udp.ReadFromUDP(buf)
	if err != nil {
		return nil, nil, err
	}

	pkt := c.pool.Get().(*Packet)
	defer c.pool.Put(pkt)
	pkt.Reset()

	err = pkt.unMarshal(buf[:n])
	if err != nil {
		return nil, nil, err
	}

	c.sCheck(pkt.ack)

	return pkt, addr, nil
}

func (c *Conn) WriteTo(pkt *Packet, addr *net.UDPAddr) error {

	pkt.ack = pkt.sequence

	pkt.sequence = c.sUp()

	c.ackChache.cache(addr.String(), pkt)
	data, _ := pkt.marshal()

	_, err := c.udp.WriteToUDP(data, addr)
	if err != nil {
		return err
	}
	return nil
}

func (c *Conn) Read() (*Packet, error) {
	buf := make([]byte, c.r)
	n, err := c.udp.Read(buf)
	if err != nil {
		return nil, err
	}

	pkt := c.pool.Get().(*Packet)
	defer c.pool.Put(pkt)
	pkt.Reset()

	err = pkt.unMarshal(buf[:n])
	if err != nil {
		return nil, err
	}
	c.sCheck(pkt.ack)
	return pkt, nil
}

func (c *Conn) Write(pkt *Packet) error {

	pkt.sequence = c.sUp()

	data, _ := pkt.marshal()

	_, err := c.udp.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// Sequence is local sequence for udp send
// each time send a packet increase the local sequence number
func (c *Conn) Sequence() uint32 {
	return c.lSequence
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

func (c *Conn) sCheck(s uint32) {
	c.mux.Lock()
	defer c.mux.Unlock()
	if sequenceGreaterThan(s, c.lSequence) {
		c.lSequence = s
	}
}

func (c *Conn) sUp() uint32 {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.lSequence++
	return c.lSequence
}

func (c *Conn) sDown() uint32 {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.lSequence--
	return c.lSequence
}

func checkNewwork(n string) error {
	switch n {
	case "udp", "udp4", "udp6":
		return nil
	}
	return errors.New("sanhua: error nework mustbe \"udp\" \"udp4\" or \"udp6\" .")
}

func sequenceGreaterThan(s1, s2 uint32) bool {
	return ((s1 > s2) && (s1-s2 <= max_sequence/2)) ||
		((s1 < s2) && (s2-s1 > max_sequence/2))
}
