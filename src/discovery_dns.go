package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type dnsDiscovery struct {
	c         *Config
	dnsConfig *dns.ClientConfig
	nameList  []string
	hostname  string
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
	d.dnsConfig, _ = dns.ClientConfigFromFile("/etc/resolv.conf")
	d.nameList = d.dnsConfig.NameList(d.c.DiscoveryDNSName)
	d.hostname, _ = os.Hostname()
}

func (d *dnsDiscovery) lookup() (hosts []Host) {
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
	for _, server := range d.dnsConfig.Servers {
		for _, name := range d.nameList {
			c := new(dns.Client)
			m := new(dns.Msg)
			m.SetQuestion(name, dns.TypeSRV)
			m.RecursionDesired = true
			r, _, err := c.Exchange(m, server+":"+d.dnsConfig.Port)
			if err != nil {
				log.Println(err)
				continue
			}

			// On failure, we just move onto the next name
			if r.Rcode != dns.RcodeSuccess {
				if r.Rcode == dns.RcodeNameError {
					log.Printf("NXDOMAIN %s@%s:%s\n", name, server, d.dnsConfig.Port)
				} else {
					log.Println("Unknown DNS failure", r.Rcode)
				}
				continue
			}

			// As an optimization, once we correctly resolve, let's shorten
			// our namelist to the one that worked.
			if len(d.nameList) > 1 {
				d.nameList = []string{name}
			}
			// We have Extra data that matches the Answer.
			// This means the Extra data contains an A record for
			// each corresponding SRV record, so we have everything
			// we need in one query.
			if len(r.Extra) == len(r.Answer) {
				for i := 0; i < len(r.Answer); i++ {
					answer := r.Answer[i]
					extra := r.Extra[i]
					srv, _ := answer.(*dns.SRV)
					a, _ := extra.(*dns.A)

					hostname := getHostname(a.Hdr.Name)
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
							FQDN: a.Hdr.Name,
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
				panic("not implemented")
			}
			return
		}
	}
	return
}

func (d *dnsDiscovery) Loop(fn loopFn) {
	go func() {
		hosts := d.lookup()
		fn(hosts)

		ticker := time.NewTicker(d.c.DiscoveryDNSRefresh)
		for {
			select {
			case <-ticker.C:
				before := hosts
				hosts = d.lookup()
				// Only want to notify if there's a change
				// between lookups
				if !hostsEqual(before, hosts) {
					fn(hosts)
				}
			}
		}
	}()
}
