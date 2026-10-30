package bgpserver

import (
	"net/netip"
	"os"
	"syscall"
)

func setTCPMD5(fd uintptr, v6Socket bool, peer netip.Addr, key string) error {
	b, err := md5SigBytes(v6Socket, peer, key)
	if err != nil {
		return err
	}
	return os.NewSyscallError("setsockopt TCP_MD5SIG",
		syscall.SetsockoptString(int(fd), syscall.IPPROTO_TCP, tcpMD5Sig, string(b)))
}
