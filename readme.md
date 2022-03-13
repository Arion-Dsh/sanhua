## sanhua
sanhua(三花猫) is kind of cat with black, red and white color.

---
This is a mini version TCP top on UDP, but with out resend
lost packet.

As we know. TCP is reliable ordered stream. becase of this. When packet is lost, 
TCP will stop and wait for that packet to be resend. this case more packet recent 
packet must wait in aqueue until the resend packet arrives, to make sure packets 
are received in the same order they were sent.

But sometime, We don't care about reliable ordered stream we just need to know packet
is receied on other side. 

### reliability

for more reliable, we useing redundancy to defeat ack packet lost or local size sends packet is faster
than remote side send rate.

Remote side will send back ack Packet.Ack and down to 33end [ack-33, ack-1] acks on Pack.AckField.

### example 

there is a simple client side code 


	c, err := sanhua.Dial("udp", ":8000")

	if err != nil {
	    painc(err)
	}

	pkt := sanhua.NewPacket()

	pkt.Write([]byte{2, 24})
	c.Write(pkt)
	pkt.Write([]byte{14, 14})
	c.Write(pkt)

	for {
		pkt, _ := c.Read()
		fmt.Println(pkt.Ack(), "ack")
	}








