package main

import (
	"bytes"
	"fmt"

	"github.com/arion-dsh/sanhua"
)

func main() {
	c, _ := sanhua.Listen("udp", ":8000")
	i := 0
	for {
		i++
		p := make([]byte, 1024*2)
		n, _, _ := c.ReadFromUDP(p)
		// fmt.Println(n, err, addr)
		fmt.Println(n)
		buf := bytes.NewBuffer(p[:n])
		fmt.Println(buf.String())

		fmt.Println(i)
	}

}
