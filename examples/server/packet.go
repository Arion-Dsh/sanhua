package main

import (
	"fmt"

	"github.com/arion-dsh/sanhua"
)

func main() {
	c, _ := sanhua.Listen("udp", ":8000")

	for {
		pkt, addr, _ := c.PacketReadFrom()
		fmt.Println(pkt, addr)
		c.PacketWriteTo(pkt, addr)
	}

}
