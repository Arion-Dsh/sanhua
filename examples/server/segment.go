package main

import (
	"bytes"
	"fmt"

	"github.com/arion-dsh/sanhua"
)

func main() {
	c, _ := sanhua.Listen("udp", ":8000")

	for {

		p := make([]byte, 2049)
		n, addr, err := c.ReadFromUDP(p)
		fmt.Println(n, err, addr)
		buf := bytes.NewBuffer(p[:n])
		fmt.Println(buf.String())
	}

}
