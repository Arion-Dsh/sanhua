package main

import (
	"github.com/arion-dsh/sanhua"
)

func main() {
	c, _ := sanhua.Listen("udp", ":8000")
	// i := 0
	for {

		p := make([]byte, 2049)
		// n, _, _ := c.ReadFromUDP(p)
		c.ReadFromUDP(p)
		// fmt.Println(n, err, addr)
		// buf := bytes.NewBuffer(p[:n])
		// fmt.Println(buf.String()[:10])
		// i++
		// fmt.Println(i)
	}

}
