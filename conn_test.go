package sanhua

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"testing"
)

var c *Conn
var laddr, raddr *net.UDPAddr

func init() {
	go func() {

		c, _ := Listen("udp", ":8000")
		p := make([]byte, 1024*2*4)
		for {
			c.ReadFromUDP(p)
		}

	}()
	var err error
	laddr, err = net.ResolveUDPAddr("udp", ":8002")

	if err != nil {
		fmt.Println("1", err)
	}

	raddr, err = net.ResolveUDPAddr("udp", ":8000")

	if err != nil {
		fmt.Println("2", err)
	}

	c, err = DialUDP("udp", laddr, raddr)

}
func TestConnFunc(t *testing.T) {

	t.Run("sequenceGreaterThan", func(t *testing.T) {
		s := []uint32{100, 144, 15, 133, 132}
		d := make([]uint32, 5)
		copy(d[1:], s)
		sort.Slice(d, func(i, j int) bool {
			return sequenceGreaterThan(d[i], d[j])
		})
		for i, v := range []uint32{144, 133, 100, 15, 0} {
			if d[i] != v {
				t.Fatal("sequenceGreaterThan err")
			}

		}

	})

}

func BenchmarkConnWrite(b *testing.B) {

	var buf bytes.Buffer
	s := `it expects to receive, in order. If TCP does not receive an ack for a given packet, it stops and resends a packet with that sequence number again. This is exactly the behavior we want to avoid!

In our reliability system, we never resend a packet with a given sequence number. We sequence n exactly once, then we send n+1, n+2 and so on. We never stop and resend packet n if it was lost, we leave it up to the application to compose a new packet containing the data that was lost, if necessary, and this packet gets sent with a new sequence number.

Because we’re doing things differently to TCP, its now possible to have holes in the set of packets we ack, so it is no longer sufficient to just state the sequence number of the most recent packet we have received.

We need to include multiple acks per-packet.

How many acks do we need?

As mentioned previously we have the case where one side of the connection sends packets faster than the other. Let’s assume that the worst case is one side sending no less than 10 packets per-second, while the other sends no more than 30. In this case, the average number of acks we’ll need per-packet is 3, but if packets clump up a bit, we would need more. Let’s say 6-10 worst case.

What about acks that don’t get through because the packet containing the ack is lost?

To solve this, we’re going to use a classic networking strategy of using redundancy to defeat packet loss!

Let’s include 33 acks per-packet, and this isn’t just going to be up to 33, but always 33. So for any given ack we redundantly send it up to 32 additional times, just in case one packet with the ack doesn’t get through!

But how can we possibly fit 33 acks in a packet? At 4 bytes per-ack thats 132 bytes!

The trick is to represent the 32 previous acks before “ack” using a bitfield:`
	buf.WriteString(s)
	buf.WriteString(s)
	buf.WriteString(s)
	buf.WriteString(s)
	buf.WriteString(s)
	buf.WriteString(s)
	buf.WriteString(s)

	b.SetBytes(int64(buf.Len()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.WriteToUDP(buf.Bytes(), raddr)

	}
	b.StopTimer()

}
