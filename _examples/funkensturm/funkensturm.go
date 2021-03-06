/* 
 * Funkensturm, a versatile DNS proxy
 * Miek Gieben <miek@miek.nl> (c) 2011
 * GPLv2
 */

package main

import (
	"os"
	"flag"
	"fmt"
	"dns"
	"os/signal"
	"strings"
)

// Define a slice of conn for sending queries
var qr []*dns.Conn
var in chan dns.Query
var out chan dns.Query

// The configuration of Funkensturm
var f *Funkensturm

// Verbose flag
var verbose *bool

// Where does the packet come from? 
// IN: initial packet received by the Responder
// any modifications here will reflect what kind of
// pkt is sent through.
// OUT: pkt as received back from a server. Modifications here will reflect
// how the packet is send back to the original requester.
const (
	IN  = iota // set when receiving a packet
	OUT        // set when sending a packet

	OR  // chain match functions with logical 'or'
	AND // chain match functions with logical 'and'
)

// A Match function is used on a DNS packet and
// returns (a possibly modified) DNS packet. It should
// return true when the packets matches the criteria set in 
// the function.
// Op is used in chaining Match-functions together
type Match struct {
	Op   int // boolean op: OR, AND
	Func func(*dns.Msg, int) (*dns.Msg, bool)
}

// An action is something that is done with a packet. Funkensturm
// does not impose any restriction on what this can be, except that
// is must remain a valid DNS packet.
type Action struct {
	Func func(*dns.Msg, bool) *dns.Msg
}

// A complete config for Funkensturm. All matches in the Matches slice are
// chained together: incoming dns.Msg -> Match[0] -> dns.Msg -> Match[1] -> dns.Msg -> ...
// The dns.Msg output of Match[n] is the input for Match[n+1]. 
//
// The final outcome (does a packet match or not?) is calculated as follows:
// true Match[0].Op Match[0].Func() Match[1].Op Match[1].Func() ...
// The result of this matching is given to the action function(s). They can then
// decide what to do with a packet in the 'true' and in the 'false' case.
type Funkensturm struct {
	Setup   func() bool // Inital setup (for extra resolvers, or loading keys, or ...)
	Matches []Match     // Match- and modify functions
	Actions []Action    // What to do with the packets
}

func verboseprint(i *dns.Msg, msg string) {
	fmt.Printf(">>>>>> %s\n", msg)
	fmt.Printf("%v", i)
	fmt.Printf("<<<<<< %s\n\n", msg)
}

func doFunkensturm(pkt *dns.Msg) ([]byte, os.Error) {
	if *verbose {
		verboseprint(pkt, "ORIGINAL INCOMING")
	}
	// No matter what, we refuse to answer requests with the response bit set.
	if pkt.MsgHdr.Response == true {
		return nil, &dns.Error{Error: "Response bit set, not replying"}
	}

	// Loop through the Match* functions and decide what to do
	// Note the packet can be changed by these function, this 
	// change is cumulative.
	ok, ok1 := true, true
	pkt1 := pkt
	for _, m := range f.Matches {
		pkt1, ok1 = m.Func(pkt1, IN)
		switch m.Op {
		case AND:
			ok = ok && ok1
		case OR:
			ok = ok || ok1
		}
	}
	if *verbose {
		verboseprint(pkt1, "MODIFIED INCOMING")
	}

	// Loop through the Actions.Func* and do something with the
	// packet. Note there can only be one returned packet. 
	// We use 'ok' to signal what the above match did: true or false
	var resultpkt *dns.Msg
	for _, a := range f.Actions {
		resultpkt = a.Func(pkt1, ok)
	}

	if *verbose {
		verboseprint(resultpkt, "ORIGINAL OUTGOING")
	}

	// loop again for matching, but now with OUT, this is done
	// for some last minute packet changing. Note the boolean return
	// code isn't used any more, i.e No more actions are allowed
	// anymore
	pkt1 = resultpkt
	for _, m := range f.Matches {
		pkt1, _ = m.Func(pkt1, OUT)
	}

	if pkt1 == nil {
		// don't need to send something back
		return nil, nil
	}

	if *verbose {
		verboseprint(pkt1, "MODIFIED OUTGOING")
		fmt.Printf("-----------------------------------\n\n")
	}

	out, ok1 := pkt1.Pack()
	if !ok1 {
		return nil, &dns.Error{Error: "Packing packet failed"}
	}
	// Some final byte changing function here? 
	return out, nil
}

func reply(c *dns.Conn, i *dns.Msg) {
	out, err := doFunkensturm(i)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.String())
		return
	}
	if out != nil {
		c.Write(out)
	}
}

func tcp(addr string, e chan os.Error) {
        err := dns.ListenAndServeTCP(addr, reply)
	e <- err
	return
}

func udp(addr string, e chan os.Error) {
        err := dns.ListenAndServeUDP(addr, reply)
	e <- err
	return
}

func main() {
	var sserver *string = flag.String("sserver", "127.0.0.1:8053", "Set the listener address")
	var rserver *string = flag.String("rserver", "127.0.0.1:53", "Remote server address(es), seperate with commas")
	verbose = flag.Bool("verbose", false, "Print packet as it flows through") // verbose needs to be global
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	resolvers := strings.Split(*rserver, ",", -1)
	qr = make([]*dns.Resolver, len(resolvers))
	for i, ra := range resolvers {
		d := new(dns.Conn)
		d.RemoteAddr = addr
		qr[i] = d
	}

	f = funkensturm()
	ok := f.Setup()
	if !ok {
		fmt.Fprintf(os.Stderr, "Setup failed")
		return
	}

        err  := make(chan os.Error)
        go udp(*sserver, err)
	go tcp(*sserver, err)

forever:
	for {
		select {
                case e := <-err:
                        fmt.Printf("Error received, stopping: %s\n", e.String())
                        break forever
		case <-signal.Incoming:
			fmt.Printf("Signal received, stopping")
			break forever
		}
	}
	close(err)
}
