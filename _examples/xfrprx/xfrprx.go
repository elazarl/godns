package main

// Xfrprx is a proxy that intercepts notify messages
// and then performs a ixfr/axfr to get the new 
// zone contents. 
// This zone is then checked cryptographically is
// everything is correct.
// When the message is deemed correct a remote 
// server is sent a notify to retrieve the ixfr/axfr.
// If a new DNSKEY record is seen for the apex and
// it validates it writes this record to disk and
// this new key will be used in future validations.

import (
	"os"
	"os/signal"
	"fmt"
	"dns"
)

// Static amount of RRs...
type zone struct {
	name string
	rrs  [10000]dns.RR
	size int
        correct bool
}

var Zone zone

func handle(d *dns.Conn, i *dns.Msg) {
	if i.MsgHdr.Response == true {
		return
	}
	if err := handleNotify(d, i); err != nil {
                fmt.Printf("err %v\n", err)
        }
//        handleNotifyOut("127.0.0.1:53") // 
	if err := handleXfrOut(d, i); err != nil {
                fmt.Printf("err %v\n", err)
        }
        if Zone.name != "" {
                // We have transfered a zone and can check it. For now assume ok.
                Zone.correct = false
        }
}

func listen(tcp string, addr string, e chan os.Error) {
	switch tcp {
	case "tcp":
		err := dns.ListenAndServeTCP(addr, handle)
		e <- err
	case "udp":
		err := dns.ListenAndServeUDP(addr, handle)
		e <- err
	}
}

func query(tcp string, e chan os.Error) {
        switch tcp {
        case "tcp":
                err := dns.QueryAndServeTCP(dns.HandleQuery)
                e <- err
        case "udp":
                err := dns.QueryAndServeUDP(dns.HandleQuery)
                e <- err
        }
}

func main() {
	err := make(chan os.Error)

	// Outgoing queries
        dns.InitQueryChannels()
	go query("tcp", err)
        go query("udp", err)

	// Incoming queries
	go listen("tcp", "127.0.0.1:8053", err)
	go listen("tcp", "[::1]:8053", err)
	go listen("udp", "127.0.0.1:8053", err)
	go listen("udp", "[::1]:8053", err)

forever:
	for {
		select {
		case e := <-err:
			fmt.Printf("Error received, stopping: %s\n", e.String())
			break forever
		case <-signal.Incoming:
			fmt.Printf("Signal received, stopping\n")
			break forever
                case q := <-dns.QueryReply:
                        fmt.Printf("Query received:\n%v\n", q.Reply)
		}
	}
	close(err)
}
