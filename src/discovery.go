package main

import (
	"fmt"
)

type Host struct {
	Name string
	FQDN string
	IP   string
	Port int
}

func (h Host) String() string {
	return fmt.Sprintf("Name=%s FQDN=%s IP=%s Port=%d", h.Name, h.FQDN, h.IP, h.Port)
}

func (h Host) IsEmpty() bool {
	return h.IP == ""
}

func hostsEqual(a, b []Host) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

type loopFn func([]Host)

type Discovery interface {
	Init(*Config)
	Loop(loopFn)
}

type noopDiscovery struct{}

func (d *noopDiscovery) Init(c *Config) {}
func (d *noopDiscovery) Loop(fn loopFn) { go func() { fn([]Host{}); select {} }() }

func NewDiscovery(c *Config) Discovery {
	var d Discovery

	switch c.DiscoveryMethod {
	case "dns":
		d = &dnsDiscovery{}
	default:
		panic("not implemented: " + string(c.DiscoveryMethod))
	}

	d.Init(c)
	return d
}
