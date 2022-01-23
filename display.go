package main

import (
	"tailscale.com/util/dnsname"
)

type HostName string

func (hn HostName) String() string {
	return string(hn)
}

type DNSName string

func (dn DNSName) String() string {
	return string(dn)
}

type Name interface {
	String() string
}

// dnsOrQuoteHostname follows the logic at
// https://github.com/tailscale/tailscale/blob/v1.18.2/cmd/tailscale/cli/status.go#L197
func dnsOrQuoteHostname(dnsSuffix string, peer RawMachine) Name {
	baseName := dnsname.TrimSuffix(peer.DNSName, dnsSuffix)
	if baseName != "" {
		return DNSName(baseName)
	}
	return HostName(dnsname.SanitizeHostname(peer.HostName))
}
