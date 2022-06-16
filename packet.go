package sanhua

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"time"
)

var (
	ErrPacketBodyNil error = errors.New("Packet body nil")
	packetPool       sync.Pool
)

func init() {
	packetPool = sync.Pool{
		New: func() interface{} {
			return &Packet{
				done: make(chan struct{}),
			}
		},
	}
}

// Packet implements generic packet-oriented .
type Packet struct {
	proto    byte
	sequence uint32
	ack      uint32
	ackField uint32
	body     bytes.Buffer

	addr *net.UDPAddr

	t    time.Time
	done chan struct{}
}

//NewPacket create a Packet. Always use this method. do not use new(Packet)
func NewPacket() *Packet {
	return &Packet{
		done: make(chan struct{}),
	}
}

// Len returns number of packet's bytes.
func (pkt *Packet) Len() int {
	return pkt.body.Len()
}

//Sequence is remote send sequence.
func (pkt *Packet) Sequence() uint32 {
	return pkt.sequence
}

// Ack local send sequence received on remote side
func (pkt *Packet) Ack() uint32 {
	return pkt.ack
}

// AckField using redundancy to defeat ack packet loss.
// from ack-1 down to ack-33
func (pkt *Packet) AckField() []uint32 {
	f := []uint32{}
	for i := uint32(0); i < 32; i++ {
		if 1<<i&pkt.ackField != 0 {
			f = append(f, pkt.ack-i-1)
		}
	}
	return f
}

//Write implements io.Writer
func (pkt *Packet) Write(p []byte) (int, error) {
	return pkt.body.Write(p)
}

// Read  implements io.Reader
func (pkt *Packet) Read(p []byte) (int, error) {
	return pkt.body.Read(p)
}

// Body represents packet body.
func (pkt *Packet) Body() []byte {
	return pkt.body.Bytes()
}

func (pkt *Packet) Wait() {
	<-pkt.done
}

func (pkt *Packet) Done() {
	pkt.done <- struct{}{}
}

// Reset resets the Pact to be empty,
//but it retains the underlying storage for use by future writers.
func (pkt *Packet) Reset() {
	pkt.sequence = 0
	pkt.ack = 0
	pkt.ackField = 0
	pkt.body.Reset()
}

func (pkt *Packet) marshalHeader() ([]byte, error) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, pkt.proto)
	binary.Write(buf, binary.BigEndian, pkt.sequence)
	binary.Write(buf, binary.BigEndian, pkt.ack)
	binary.Write(buf, binary.BigEndian, pkt.ackField)
	b := buf.Bytes()
	return b, nil
}

func (pkt *Packet) marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, pkt.proto)
	binary.Write(buf, binary.BigEndian, pkt.sequence)
	binary.Write(buf, binary.BigEndian, pkt.ack)
	binary.Write(buf, binary.BigEndian, pkt.ackField)

	_, err := pkt.body.WriteTo(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (pkt *Packet) unMarshal(buf []byte) error {
	data := bytes.NewBuffer(buf)
	binary.Read(data, binary.BigEndian, &pkt.proto)
	binary.Read(data, binary.BigEndian, &pkt.sequence)
	binary.Read(data, binary.BigEndian, &pkt.ack)
	binary.Read(data, binary.BigEndian, &pkt.ackField)
	data.WriteTo(&pkt.body)
	return nil
}

type ackField struct {
	update time.Time
	acks   []uint32
}

type ackChache struct {
	mux    sync.Mutex
	fields map[string]*ackField
	done   chan interface{}
}

func newAckCache() *ackChache {
	c := &ackChache{fields: map[string]*ackField{}}

	go func(c *ackChache) {
		t := time.NewTicker(5 * time.Minute)
		for {
			select {
			case <-c.done:
				return
			case n := <-t.C:
				c.flush(n)
			}
		}

	}(c)

	return c
}

func (af *ackChache) cache(k string, ack uint32) (uint32, bool) {
	af.mux.Lock()
	defer af.mux.Unlock()
	f, ok := af.fields[k]
	if !ok {
		f = &ackField{update: time.Now(), acks: make([]uint32, 0, 2000)}
		af.fields[k] = f
	}
	has := af.checkAck(f.acks, ack)

	//check down to 32nd ack
	var fields uint32
	for i := uint32(1); i < 33; i++ {
		ok := af.checkAck(f.acks, ack-i)
		if ok {
			// fields |= 1 << (ack - a - 1)
			fields |= 1 << (i - 1)
		}
	}

	//max cache 2000 acks
	if len(f.acks) == cap(f.acks) {
		s := make([]uint32, 0, 2000)
		s = append(s, f.acks[2000-33:]...)
		f.acks = s
	}

	f.acks = append(f.acks, ack)

	// seemed sort is not necessary.
	// [3, 2, 1]
	/*
		sort.Slice(n,
			func(i, j int) bool {
				return sequenceGreaterThan(n[i], n[j])
			},
		)
	*/
	return fields, has
}

func (af *ackChache) checkAck(fs []uint32, a uint32) bool {
	for _, v := range fs {
		if a == v {
			return true
		}
	}

	return false
}

func (af *ackChache) flush(now time.Time) {

	for k, v := range af.fields {
		if v.update.Add(5 * time.Minute).Before(now) {
			delete(af.fields, k)
		}

	}
}

func (af *ackChache) close() {
	af.done <- struct{}{}
}
