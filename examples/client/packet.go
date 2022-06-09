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
	c.PacketWriteTo(pkt, raddr)
	pkt.Write([]byte{2})
	c.PacketWriteTo(pkt, raddr)

	for {
		pkt, _, _ := c.PacketReadFrom()
		fmt.Println(pkt.AckField(), "pkt")
	}
}
