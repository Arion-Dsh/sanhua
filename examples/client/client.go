package main

import (
	"fmt"
	"net"

	"github.com/arion-dsh/sanhua"
)

func main() {

	laddr, err := net.ResolveUDPAddr("udp", ":8001")

	if err != nil {
		fmt.Println("1", err)
	}

	raddr, err := net.ResolveUDPAddr("udp", ":8000")

	if err != nil {
		fmt.Println("2", err)
	}

	c, err := sanhua.DialUDP("udp", laddr, raddr)

	if err != nil {
		fmt.Print(err)
	}

	pkt := sanhua.NewPacket()

	pkt.Write([]byte{1})

	c.Write(pkt)
	pkt.Write([]byte{2, 24})
	c.Write(pkt)
	pkt.Write([]byte{14, 14})
	c.Write(pkt)

	for {
		pkt, _ := c.Read()
		fmt.Println(pkt.Ack(), "ack")
	}
}
