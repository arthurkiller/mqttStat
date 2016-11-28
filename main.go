package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"

	"github.com/arthurkiller/mqttState/packets"
)

var tcpconn = func(address string) (net.Conn, time.Duration, error) {
	var err error
	s := time.Now()
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	t := time.Since(s)
	if err != nil {
		log.Println(err)
		return nil, time.Duration(0), err
	}
	return conn, t, nil
}

var dnslookup = func(address string) (string, time.Duration, error) {
	var err error
	s := time.Now()
	ns, err := net.LookupHost("www.baidu.com")
	t := time.Since(s)
	if err != nil {
		log.Println(err)
		return "", time.Duration(0), err
	}
	return ns[0], t, nil
}

var tlshandshake = func(conn net.Conn, cfg *tls.Config) (net.Conn, time.Duration, error) {
	var err error
	conntls := tls.Client(conn, cfg)
	s := time.Now()
	err = conntls.Handshake()
	t := time.Since(s)
	if err != nil {
		log.Println(err)
		return nil, time.Duration(0), err
	}
	return conntls, t, nil
}

var httprequest = func() {}

var buildMQTTpacket = func(name, passwd string) packets.ControlPacket {
	mp := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	mp.ClientIdentifier = "test"
	mp.ProtocolName = "MQTT"
	mp.ProtocolVersion = byte(4)
	mp.Username = name
	mp.Qos = 1
	mp.Keepalive = uint16(1)
	mp.CleanSession = true
	mp.WillFlag = false
	mp.WillRetain = false
	mp.Dup = false
	mp.PasswordFlag = false
	if passwd != "" {
		mp.PasswordFlag = true
		mp.Password = []byte(passwd)
	}
	mp.Retain = false
	return mp
}

func main() {
	addr := flag.String("server", "tls://172.16.200.11:1884", "set for the addr with the style tcp:// | tls:// | http:// | https://")
	num := flag.Int("count", 1, "the testing secquence times")
	port := flag.String("port", "1883", "the mqtt broker port")
	name := flag.String("name", "test", "set the name for mqtt")
	passwd := flag.String("passwd", "", "set the passwd if needed")
	ca := flag.String("ca", "", "set the certific key path")
	pem := flag.String("pem", "", "set the certific pem path")
	tcpfilter := flag.Int("tcpfilter", 100, "the filter of tcp connecting cost")
	tlsfilter := flag.Int("tlsfilter", 150, "the filter of tls connecting cost")
	mqttfilter := flag.Int("mqttfilter", 100, "the filter of mqtt connecting cost")
	flag.Parse()
	_ = ca

	ss := strings.Split(*addr, "://")
	if len(ss) == 1 {
		flag.Usage()
		return
	}
	if len(strings.Split(ss[1], ":")) != 1 {
		*port = ss[1]
	}
	var server string = ss[1] + ":" + *port

	var sumdns time.Duration
	var sumtcp time.Duration
	var sumtls time.Duration
	var summqtt time.Duration

	var countdns int
	var counttcp int
	var counttls int
	var countmqtt int

	var withTLS bool = false
	var needDNS bool = true
	var tlsConfig = &tls.Config{}

	if ss[0] == "https" || ss[0] == "tls" {
		withTLS = true
	}

	if ss[1][0] <= 57 && ss[1][0] >= 48 {
		needDNS = false
	}

	if withTLS {
		if *pem != "" && *ca != "" {
			ca_b, err := ioutil.ReadFile(*pem)
			if err != nil {
				return
			}
			cas, err := x509.ParseCertificate(ca_b)
			if err != nil {
				return
			}
			priv_b, err := ioutil.ReadFile(*ca)
			if err != nil {
				return
			}
			priv, err := x509.ParsePKCS1PrivateKey(priv_b)
			if err != nil {
				return
			}
			pool := x509.NewCertPool()
			pool.AddCert(cas)
			cert := tls.Certificate{
				Certificate: [][]byte{ca_b},
				PrivateKey:  priv,
			}

			tlsConfig = &tls.Config{
				ClientAuth:   tls.VerifyClientCertIfGiven,
				Certificates: []tls.Certificate{cert},
			}
		} else {
			tlsConfig = &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
		}
	} else {
		tlsConfig = &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	}
	mp := buildMQTTpacket(*name, *passwd)
	//
	if needDNS {
		ts, _, _ := dnslookup(ss[1])
		fmt.Println(ts)
	}

	for i := 1; i <= *num; i++ {
		var t0 time.Duration = 0
		if needDNS {
			s, t, err := dnslookup(ss[1])
			if err != nil {
				log.Fatalln("error in lookup dns", err)
			}
			t0 = t
			server = s + ":" + *port
			sumdns += t0
			countdns++
		}

		//do tcp cost test
		conn, t1, err := tcpconn(server)
		if err != nil {
			log.Fatalln("error in tcp conn", err)
			t1 = 0
			continue
		} else {
			sumtcp += t1
			counttcp++
		}

		//do tls cost test
		var t2 time.Duration = 0
		if withTLS {
			conntls, t, err := tlshandshake(conn, tlsConfig)
			if err != nil {
				log.Fatalln("error in tls handshake", err)
			} else {
				conn = conntls
				t2 = t
				sumtls += t
				counttls++
			}
		}

		//TODO with http

		//do mqtt test
		t := time.Now()
		err = mp.Write(conn)
		if err != nil {
			log.Fatalln("error in write conn packet", err)
		}
		ca, err := packets.ReadPacket(conn)
		t3 := time.Since(t)
		if _, ok := ca.(*packets.ConnackPacket); err != nil || !ok {
			log.Fatalln("error in read ack", err, ca)
			t3 = 0
		} else {
			summqtt += t3
			countmqtt++
		}

		//do print
		trans := func(filter int) time.Duration {
			return time.Duration(time.Millisecond * time.Duration(filter))
		}

		if needDNS {
			if withTLS {
				if t1 > trans(*tcpfilter) || t2 > trans(*tlsfilter) || t3 > trans(*mqttfilter) {
					fmt.Printf("%c[1;40;31mIn connection sequence%4v: costs %12v %12v %12v %12v %c[0m\n", 0x1B, i, t0.String(), t1.String(), t2.String(), t3.String(), 0x1B)
				} else {
					fmt.Printf("In connection sequence%4v: costs %12v %12v %12v %12v \n", i, t0.String(), t1.String(), t2.String(), t3.String())
				}
			} else {
				if t1 > trans(*tcpfilter) || t2 > trans(*tlsfilter) || t3 > trans(*mqttfilter) {
					fmt.Printf("%c[1;40;31mIn connection sequence%4v: costs %12v %12v %12v %c[0m\n", 0x1B, i, t0.String(), t1.String(), t3.String(), 0x1B)
				} else {
					fmt.Printf("In connection sequence%4v: costs %12v %12v %12v \n", i, t0.String(), t1.String(), t3.String())
				}
			}
		} else {
			if withTLS {
				if t1 > trans(*tcpfilter) || t2 > trans(*tlsfilter) || t3 > trans(*mqttfilter) {
					fmt.Printf("%c[1;40;31mIn connection sequence%4v: costs %12v %12v %12v %c[0m\n", 0x1B, i, t1.String(), t2.String(), t3.String(), 0x1B)
				} else {
					fmt.Printf("In connection sequence%4v: costs %12v %12v %12v \n", i, t1.String(), t2.String(), t3.String())
				}
			} else {
				if t1 > trans(*tcpfilter) || t2 > trans(*tlsfilter) || t3 > trans(*mqttfilter) {
					fmt.Printf("%c[1;40;31mIn connection sequence%4v: costs %12v %12v %c[0m\n", 0x1B, i, t1.String(), t3.String(), 0x1B)
				} else {
					fmt.Printf("In connection sequence%4v: costs %12v %12v \n", i, t1.String(), t3.String())
				}
			}
		}
		conn.Close()
	}

	var avgdns int64 = 0
	var avgtls int64 = 0

	//summary
	if needDNS {
		fmt.Println("Avg DNS lookup cost:", (sumdns / time.Duration(countdns)).String())
		avgdns = (sumdns / time.Duration(countdns)).Nanoseconds() / 1000000
	}
	fmt.Println("Avg tcp connection cost:", (sumtcp / time.Duration(counttcp)).String())
	avgtcp := (sumtcp / time.Duration(counttcp)).Nanoseconds() / 1000000
	if withTLS {
		fmt.Println("Avg tls handshake cost:", (sumtls / time.Duration(counttls)).String())
		avgtls = (sumtls / time.Duration(counttls)).Nanoseconds() / 1000000
	}
	fmt.Println("Avg mqtt connection cost:", (summqtt / time.Duration(countmqtt)).String())
	avgmqtt := (summqtt / time.Duration(countmqtt)).Nanoseconds() / 1000000

	sumt := avgdns + avgtcp + avgtls + avgmqtt
	avgdns = int64(float32(avgdns) / float32(sumt) * 50)
	avgtcp = int64(float32(avgtcp) / float32(sumt) * 50)
	avgtls = int64(float32(avgtls) / float32(sumt) * 50)
	avgmqtt = int64(float32(avgmqtt) / float32(sumt) * 50)
	var i int64 = 0
	var sb string = ""
	fmt.Println()
	if needDNS {
		fmt.Print("avg DNS lookup cost | ")
		for i = 0; i < avgdns; i++ {
			sb += "*"
		}
		sb += "|"
	}
	fmt.Print("avg tcp connect cost | ")
	for i = 0; i < avgtcp; i++ {
		sb += "*"
	}
	sb += "|"
	if withTLS {
		fmt.Print("avg tls handshake cost | ")
		for i = 0; i < avgtls; i++ {
			sb += "*"
		}
		sb += "|"
	}
	fmt.Print("avg mqtt connect cost \n")
	for i = 0; i < avgmqtt; i++ {
		sb += "*"
	}
	fmt.Println(sb)
}
