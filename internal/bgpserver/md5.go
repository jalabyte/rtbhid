package bgpserver

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"syscall"
)

// Linux struct tcp_md5sig: sockaddr_storage(128) flags(1) prefixlen(1)
// keylen(2) ifindex(4) key(80).
const (
	tcpMD5Sig    = 14 // TCP_MD5SIG
	tcpMD5SigLen = 216
	tcpMD5MaxKey = 80
	afInet       = 2  // Linux AF_INET
	afInet6      = 10 // Linux AF_INET6
)

// md5SigBytes builds struct tcp_md5sig for peer. An empty key removes it.
// v6Socket is true for AF_INET6 (incl. dual-stack) sockets, where IPv4 peers
// must be given as v4-mapped addresses.
func md5SigBytes(v6Socket bool, peer netip.Addr, key string) ([]byte, error) {
	if len(key) > tcpMD5MaxKey {
		return nil, errors.New("TCP MD5 key longer than 80 bytes")
	}
	b := make([]byte, tcpMD5SigLen)
	switch {
	case v6Socket:
		binary.NativeEndian.PutUint16(b[0:2], afInet6)
		a := peer.As16() // IPv4 becomes ::ffff:a.b.c.d
		copy(b[8:24], a[:])
	case peer.Unmap().Is4():
		binary.NativeEndian.PutUint16(b[0:2], afInet)
		a := peer.Unmap().As4()
		copy(b[4:8], a[:])
	default:
		return nil, errors.New("IPv6 peer on an IPv4-only socket")
	}
	binary.NativeEndian.PutUint16(b[130:132], uint16(len(key)))
	copy(b[136:], key)
	return b, nil
}

// setMD5 applies the key to the socket behind c; network is "tcp4"/"tcp6".
func setMD5(c syscall.RawConn, network string, peer netip.Addr, key string) error {
	var serr error
	if err := c.Control(func(fd uintptr) { serr = setTCPMD5(fd, network == "tcp6", peer, key) }); err != nil {
		return err
	}
	return serr
}
