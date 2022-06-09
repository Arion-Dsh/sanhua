package main

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/arion-dsh/sanhua"
)

func main() {

	laddr, err := net.ResolveUDPAddr("udp", ":8002")

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

	var buf bytes.Buffer
	buf.WriteString("123")
	t := time.Now()
	c.WriteToUDP(buf.Bytes(), raddr)
	n := time.Now()
	fmt.Println(n.Sub(t))

}
