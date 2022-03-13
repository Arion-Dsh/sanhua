package main

import (
	"fmt"

	"github.com/arion-dsh/sanhua"
)

func main() {
	conn, _ := sanhua.Listen("udp", ":8000")
	for {
		pkt, addr, err := conn.ReadFrom()
		if err != nil {
			fmt.Print(err)
		}
		conn.WriteTo(pkt, addr)
	}
}
