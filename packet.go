package sanhua

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"
)

var (
	ErrPacketBodyNil error = errors.New("Packet body nil")
)

// Packet implements generic packet-oriented .
type Packet struct {
	sequence uint32
	ack      uint32
	ackField uint32
	body     *bytes.Buffer
}

//NewPacket create a Packet. Always use this method. do not use new(Packet)
func NewPacket() *Packet {
	return &Packet{body: new(bytes.Buffer)}
}

// Size return packet's bit size.
func (pkt *Packet) Size() int {
	return 96 + pkt.body.Len()*4
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
func (pkt *Packet) Body() io.ReadWriter {
	return pkt.body
}

// Reset resets the Pact to be empty,
//but it retains the underlying storage for use by future writes.
func (pkt *Packet) Reset() {
	pkt.sequence = 0
	pkt.ack = 0
	pkt.ackField = 0
	pkt.body.Reset()
}

func (pkt *Packet) marshal() ([]byte, error) {
	if pkt.body == nil {
		return nil, ErrPacketBodyNil
	}
	buf := new(bytes.Buffer)
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
	var p Packet
	data := bytes.NewBuffer(buf)
	binary.Read(data, binary.BigEndian, &p.sequence)
	binary.Read(data, binary.BigEndian, &p.ack)
	binary.Read(data, binary.BigEndian, &p.ackField)
	p.body = data
	pkt = &p
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

func (af *ackChache) cache(k string, pkt *Packet) {
	af.mux.Lock()
	defer af.mux.Unlock()
	f, ok := af.fields[k]
	if !ok {
		f = &ackField{update: time.Now()}
		af.fields[k] = f
	}

	//check down to 32nd ack
	for i := uint32(1); i < 33; i++ {
		a, ok := af.checkAck(f.acks, pkt.ack-i)
		if ok {
			pkt.ackField |= 1 << (pkt.ack - a - 1)
		}
	}

	//max cache 100 acks
	size := len(f.acks) + 1
	if size > 100 {
		size = 100
	}
	n := make([]uint32, size)
	n[0] = pkt.ack
	copy(n[1:], f.acks)

	// seemed sort is not necessary.
	// [3, 2, 1]
	/*
		sort.Slice(n,
			func(i, j int) bool {
				return sequenceGreaterThan(n[i], n[j])
			},
		)
	*/
	f.acks = n
}

func (af *ackChache) checkAck(fs []uint32, a uint32) (uint32, bool) {
	for _, v := range fs {
		if a == v {
			return v, true
		}
	}

	return 0, false
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
