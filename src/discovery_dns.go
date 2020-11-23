package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"fmt"

	"github.com/miekg/dns"
)

type dnsDiscovery struct {
	c         *Config
	dnsConfig *dns.ClientConfig
	nameList  []string
	hostname  string
	net       string
}

// We assume the hostname portion is the first part before the .
// why would you give yourself a hostname that contains a .
func getHostname(h string) string {
	bits := strings.SplitN(h, ".", 2)
	return bits[0]
}

// We assume our hostnames are in the form of [name]-[number]
// in which case, the number is the id of the node within the cluster.
func idFromHostname(h string) int {
	bits := strings.Split(h, "-")
	id, _ := strconv.Atoi(bits[len(bits)-1])
	return id
}

func (d *dnsDiscovery) Init(c *Config) {
	d.c = c
	d.dnsConfig, _ = dns.ClientConfigFromFile(c.DiscoveryDNSResolvConf)
	d.dnsConfig.Port = strconv.Itoa(c.DiscoveryDNSPort)
	d.nameList = d.dnsConfig.NameList(d.c.DiscoveryDNSName)
	d.hostname, _ = os.Hostname()
	if c.DiscoveryDNSUseTCP {
		d.net = "tcp"
	} else {
		d.net = "udp"
	}
}

func (d *dnsDiscovery) lookup() (hosts []Host, reterr error) {
	// Welcome to the meat.
	// We rely on querying an SRV record to build up
	// a list of hosts to be sent along to haproxy.
	// This needs to be done in order for consistent hashing to work
	// correctly in haproxy. Haproxy maintains a consistent hash based
	// on the order of hosts within it's slots. So hosts need to be
	// placed in the same consistent slots in order to work correctly.
	// We rely on hostnames in the pattern of [name]-[id], and this id
	// ends up corresponding to which haproxy slot. So abc-0 goes to
	// slot 0, abc-1 goes to slot 1, etc.

	// To correctly resolve DNS, we need to iterate over each possible
	// nameserver based on /etc/resolv.conf, and for each nameserver,
	// query for each possible name in the nameList. This includes each
	// permutation of the search domain and ndots if applicable.
	hosts = make([]Host, d.c.HaproxySlots)
	reterr = nil

	for _, server := range d.dnsConfig.Servers {
		for _, name := range d.nameList {
			c := &dns.Client{
				Net: d.net,
			}
			m := new(dns.Msg)
			m.SetQuestion(name, dns.TypeSRV)
			m.RecursionDesired = true
			dnsHost := server + ":" + d.dnsConfig.Port
			r, _, err := c.Exchange(m, dnsHost)
			if err != nil {
				log.Println(err)
				reterr = err
				continue
			}

			if r.MsgHdr.Truncated {
				log.Println("! DNS Query was truncated")
			}

			// On failure, we just move onto the next name
			if r.Rcode != dns.RcodeSuccess {
				if r.Rcode == dns.RcodeNameError {
					log.Printf("NXDOMAIN %s@%s:%s\n", name, server, d.dnsConfig.Port)
					reterr = fmt.Errorf("NXDOMAIN %s@%s:%s", name, server, d.dnsConfig.Port)
				} else {
					log.Println("Unknown DNS failure", r.Rcode)
					reterr = fmt.Errorf("Unknown DNS failure. Code: %d", r.Rcode)
				}
				continue
			}

			// As an optimization, once we correctly resolve, let's shorten
			// our namelist to the one that worked.
			if len(d.nameList) > 1 {
				d.nameList = []string{name}
			}

			// Map all of extra A records so we can grab them from SRV
			extraAs := make(map[string]*dns.A)
			for _, answer := range r.Extra {
				if a, ok := answer.(*dns.A); ok {
					extraAs[a.Header().Name] = a
				}
			}

			// We have Extra data that matches the Answer.
			// This means the Extra data contains an A record for
			// each corresponding SRV record, so we have everything
			// we need in one query.
			if len(extraAs) == len(r.Answer) {
				for i := 0; i < len(r.Answer); i++ {
					answer := r.Answer[i]
					srv, _ := answer.(*dns.SRV)
					a := extraAs[srv.Target]

					hostname := getHostname(a.Header().Name)
					id := idFromHostname(hostname)
					ip := a.A.String()

					// As an optimization, if we know this is for our local peer,
					// we route it to localhost.
					if hostname == d.hostname {
						ip = "127.0.0.1"
					}

					if id < d.c.HaproxySlots {
						hosts[id] = Host{
							Name: hostname,
							FQDN: a.Header().Name,
							IP:   ip,
							Port: int(srv.Port),
						}
					}
				}
			} else {
				// In this case, we got an SRV response, but no Extra data,
				// if this becomes an issue, we'd need to implement resolving
				// the A records for each host here to build up the full
				// mapping. But until then, I think our DNS servers always attach
				// what we need.
				log.Println(r.Answer)
				log.Println(extraAs)
				log.Println(r.Extra)
				panic("not implemented")
			}
			// We end up here only after successful resolution
			// therefore nullify previous errors
			reterr = nil
			return
		}
	}
	return
}

func (d *dnsDiscovery) Loop(fn loopFn) {
	go func() {
		hosts, err := d.lookup()
		if err != nil {
			// This is the first time we attempt DNS lookup so it
			// is ok to ignore error and jump into the loop with
			// empty hosts as they will eventually fill with next
			// lookup
			log.Println("error occured doing the first lookup, ignoring")
		}
		fn(hosts)

		ticker := time.NewTicker(d.c.DiscoveryDNSRefresh)
		for {
			select {
			case <-ticker.C:
				before := hosts
				hosts, err = d.lookup()
				if err != nil {
					continue
				}
				// Only want to notify if there's a change
				// between lookups
				if !hostsEqual(before, hosts) {
					fn(hosts)
				}
			}
		}
	}()
}
