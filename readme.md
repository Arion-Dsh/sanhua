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

1.  Useing PacketWriteTo 
 for more reliable, we useing redundancy to defeat ack packet lost or local size sends packet is faster
than remote side send rate. 
Remote side will send back ack Packet.Ack and down to 33end [ack-33, ack-1] acks on Pack.AckField.

2. Using WriteTo, WriteToUDP
  it will split p([]byte) to several packets and  block until return acks. if longer than RTT. It will
  resend the unreceived packet unless timeout(1 second).  use PacketWriteTo if you wan write your resend algorithm.


### example 

there is a simple  code 

	// server side 
	go func() {

		c, _ := Listen("udp", ":8000")
		p := make([]byte, 1024*2*4)
		for {
			c.ReadFromUDP(p)
		}

	}()

	// client 
	laddr, _ = net.ResolveUDPAddr("udp", ":8002")


	raddr, _ = net.ResolveUDPAddr("udp", ":8000")


	c, _ := DialUDP("udp", laddr, raddr)

	c.WriteToUDP([]byte{}, raddr)





	







