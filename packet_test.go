package sanhua

import (
	"fmt"
	"net"
	"testing"
)

func TestPacket(t *testing.T) {
	cache := newAckCache()
	addr1, _ := net.ResolveUDPAddr("udp", ":8000")

	checkAckField := func(a uint32, f []uint32) bool {

		ok := false

		for _, v := range f {
			if a == v {
				ok = true
			}
		}
		return ok
	}
	p1 := Packet{sequence: 1, ack: 1}
	cache.cache(addr1.String(), p1.ack)

	p3 := Packet{sequence: 3, ack: 3}
	p3.ackField, _ = cache.cache(addr1.String(), p3.ack)
	fmt.Printf("%v, %v\n", p3.ackField, p3.AckField())
	if !checkAckField(1, p3.AckField()) {
		t.Fatal("cache err line 32")
	}

	p2 := Packet{sequence: 2, ack: 2}
	p2.ackField, _ = cache.cache(addr1.String(), p2.ack)
	fmt.Printf("%v, %v\n", p2.ackField, p2.AckField())
	if checkAckField(3, p2.AckField()) {
		t.Fatal("cache err line 40")
	}

	p5 := Packet{sequence: 5, ack: 5}
	p5.ackField, _ = cache.cache(addr1.String(), p5.ack)
	fmt.Printf("%v, %v\n", p5.ackField, p5.AckField())
	if !checkAckField(3, p5.AckField()) {
		t.Fatal("cache err line 47")
	}

}

func BenchmarkCache(b *testing.B) {

	cache := newAckCache()
	addr1, _ := net.ResolveUDPAddr("udp", ":8000")
	k := addr1.String()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p1 := Packet{sequence: uint32(i), ack: uint32(i)}
		cache.cache(k, p1.ack)

	}
}

func BenchmarkMarshal(b *testing.B) {

	p := NewPacket()
	p.proto = 42
	p.sequence = 1
	p.ack = 1
	p.ackField = 0

	buf := []byte{0, 1, 0, 1, 0, 1, 0, 0, 1, 0, 1, 1, 11, 1}

	b.Run("marshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p.marshalHeader()
		}

	})
	p.Reset()
	b.Run("unMarshal", func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.unMarshal(buf)
		}

	})

}
