package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/arthurkiller/mqttState/packets"
)

func main() {
	addr := flag.String("addr", "172.16.200.11:1884", "set for the addr")
	Num := flag.Int("count", 20, "the testing times count")
	filter := flag.Int("filter", 100, "the filter of TTL")

	flag.Parse()

	var sumtcp time.Duration
	var sumtls time.Duration
	var summqtt time.Duration

	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}

	m := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	m.ClientIdentifier = "test"
	m.ProtocolName = "MQTT"
	m.ProtocolVersion = byte(4)
	m.Username = "test"
	m.Password = []byte{1, 2, 3}
	m.Keepalive = uint16(1)
	m.CleanSession = true
	m.WillFlag = false
	m.WillRetain = false

	var dailer net.Dialer
	for i := 0; i < *Num; i++ {
		//do tcp cost test
		t1 := time.Now()
		conn, err := dailer.Dial("tcp", *addr)
		tt1 := time.Since(t1)
		if err != nil {
			log.Println(err)
		}
		sumtcp += tt1

		//do tls cost test
		conntls := tls.Client(conn, tlsConfig)
		t2 := time.Now()
		err = conntls.Handshake()
		tt2 := time.Since(t2)
		if err != nil {
			log.Println(err)
		}
		sumtls += tt2

		//do mqtt test
		t3 := time.Now()
		m.Write(conntls)
		ca, err := packets.ReadPacket(conntls)
		tt3 := time.Since(t3)
		if _, ok := ca.(*packets.ConnackPacket); err != nil || !ok {
			log.Println(err)
		}
		summqtt += tt3

		//do print
		fil := time.Duration(time.Millisecond * time.Duration(*filter))
		if tt1 > fil || tt2 > (fil+time.Duration(50)*time.Millisecond) || tt3 > fil {
			fmt.Printf("%c[1;40;31mIn connection sequence%4v: costs %12v %12v %12v %c[0m\n", 0x1B, i, tt1.String(), tt2.String(), tt3.String(), 0x1B)
		} else {
			fmt.Printf("In connection sequence%4v: costs %12v %12v %12v \n", i, tt1.String(), tt2.String(), tt3.String())
		}
		conntls.Close()
		conn.Close()
	}
	fmt.Println("tcp cost:", (sumtcp / time.Duration(*Num)).String())
	fmt.Println("tls cost:", (sumtls / time.Duration(*Num)).String())
	fmt.Println("mqtt cost:", (summqtt / time.Duration(*Num)).String())
}
