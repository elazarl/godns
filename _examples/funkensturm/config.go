package main

import (
	"dns"
)

func match(m *dns.Msg, d int) (*dns.Msg, bool) {
	// Matching criteria
        switch d {
        case IN:
                // nothing
        case OUT:
                // Note that when sending back only the mangling is important
                // the actual return code of these function isn't checked by
                // funkensturm
        }

	// Packet Mangling
	switch d {
	case IN:
		// nothing
	case OUT:
		// nothing
	}
	return m, true
}

func send(m *dns.Msg, ok bool) (o *dns.Msg) {
	switch ok {
	case true, false:
                for _, r := range qr {
                        in <- Query{Msg: m, Conn: r}
                }
                return
	}
	return
}

// Return the configration
func funkensturm() *Funkensturm {
	f := new(Funkensturm)

        // Nothing to set up
	f.Setup = func() bool { return true }

        // 1 match function, use AND as op (doesn't matter in this case)
	f.Matches = make([]Match, 1)
	f.Matches[0].Op = AND
	f.Matches[0].Func = match

        // 1 action
	f.Actions = make([]Action, 1)
	f.Actions[0].Func = send
	return f
}
