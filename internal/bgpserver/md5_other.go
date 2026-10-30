//go:build !linux

package bgpserver

import (
	"errors"
	"net/netip"
)

func setTCPMD5(uintptr, bool, netip.Addr, string) error {
	return errors.New("TCP MD5 (RFC 2385) requires Linux")
}
