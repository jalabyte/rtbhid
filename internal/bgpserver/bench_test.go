package bgpserver

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"net/netip"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// Sized to production: ~160k prefixes (Komdigi 102,724 + RTBHTurk 56,438 +
// IPThreat 721 on 2026-10-30), plus 250k headroom.
var benchSizes = []int{160_000, 250_000}

// benchPrefixes mimics the feeds: mostly IPv4 /32, some /24, ~1% IPv6.
func benchPrefixes(n int, seed uint64) []string {
	r := rand.New(rand.NewPCG(seed, 1))
	v4 := func() netip.Addr {
		var a [4]byte
		binary.BigEndian.PutUint32(a[:], r.Uint32())
		return netip.AddrFrom4(a)
	}
	out := make([]string, 0, n)
	for range n {
		switch x := r.IntN(100); {
		case x < 1:
			var a [16]byte
			binary.BigEndian.PutUint64(a[:8], 0x20010db8<<32|r.Uint64()&0xffffffff)
			binary.BigEndian.PutUint64(a[8:], r.Uint64())
			out = append(out, netip.AddrFrom16(a).String())
		case x < 3:
			out = append(out, netip.PrefixFrom(v4(), 24).Masked().String())
		default:
			out = append(out, v4().String())
		}
	}
	return out
}

func benchServer(b *testing.B) *Server {
	b.Helper()
	srv := New(Config{
		LocalASN:       65000,
		ListenAddr:     "127.0.0.1:0",
		HoldTime:       90 * time.Second,
		NexthopV4:      netip.MustParseAddr("192.0.2.1"),
		NexthopV6:      netip.MustParseAddr("2001:db8::1"),
		EnableIPv6:     true,
		StdCommunities: []Community{{High: 65535, Low: 666}},
		Logger:         log.New(io.Discard, "", 0),
	})
	if err := srv.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(srv.Stop)
	return srv
}

// benchPeer is a downstream router: handshakes, then counts what it receives.
type benchPeer struct {
	ann, wd, msgs, bytes atomic.Int64
}

// pipeSession attaches an in-memory peer (one 127.0.0.1 address can't carry
// many distinct sessions on macOS).
func pipeSession(b *testing.B, srv *Server, i int) *benchPeer {
	b.Helper()
	ours, theirs := net.Pipe()
	b.Cleanup(func() { _ = theirs.Close() })
	sess := newSession(srv, ours, &PeerConfig{Name: fmt.Sprintf("p%d", i), Neighbor: "192.0.2.2"}, false)
	go srv.runSession(sess)

	p := &benchPeer{}
	if _, err := readMessage(theirs); err != nil { // server OPEN
		b.Fatal(err)
	}
	if _, err := theirs.Write(encodeOpen(65001, 90, 0x0a000002, true)); err != nil {
		b.Fatal(err)
	}
	go func() {
		for {
			m, err := readMessage(theirs)
			if err != nil {
				return
			}
			if m.Type != msgUpdate {
				continue
			}
			p.msgs.Add(1)
			p.bytes.Add(int64(headerLen + len(m.Body)))
			if u, err := decodeUpdate(m.Body); err == nil {
				p.ann.Add(int64(len(u.Announced) + len(u.MPAnnounced)))
				p.wd.Add(int64(len(u.Withdrawn) + len(u.MPWithdrawn)))
			}
		}
	}()
	if _, err := theirs.Write(encodeKeepalive()); err != nil {
		b.Fatal(err)
	}
	return p
}

func (p *benchPeer) waitFor(b *testing.B, ann, wd int64) {
	b.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for p.ann.Load() < ann || p.wd.Load() < wd {
		if time.Now().After(deadline) {
			b.Fatalf("timeout: ann %d/%d wd %d/%d", p.ann.Load(), ann, p.wd.Load(), wd)
		}
		time.Sleep(time.Millisecond)
	}
}

// symDiff counts prefixes in exactly one snapshot (withdrawn + announced).
func symDiff(a, c routeSnapshot) int64 {
	n := int64(0)
	in4 := map[v4Prefix]int{}
	for _, p := range a.v4 {
		in4[p]++
	}
	for _, p := range c.v4 {
		in4[p]--
	}
	for _, v := range in4 {
		n += int64(v * v)
	}
	in6 := map[v6Prefix]int{}
	for _, p := range a.v6 {
		in6[p]++
	}
	for _, p := range c.v6 {
		in6[p]--
	}
	for _, v := range in6 {
		n += int64(v * v)
	}
	return n
}

func snapLen(prefixes []string) int64 {
	s := buildSnapshot(prefixes)
	return int64(len(s.v4) + len(s.v6))
}

func BenchmarkBuildSnapshot(b *testing.B) {
	for _, n := range benchSizes {
		prefixes := benchPrefixes(n, 1)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buildSnapshot(prefixes)
			}
		})
	}
}

// BenchmarkSetRoutesUnchanged is the steady state: every reconcile resends the
// same list.
func BenchmarkSetRoutesUnchanged(b *testing.B) {
	for _, n := range benchSizes {
		prefixes := benchPrefixes(n, 1)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			srv := benchServer(b)
			p := pipeSession(b, srv, 0)
			srv.SetRoutes(prefixes)
			p.waitFor(b, snapLen(prefixes), 0)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				srv.SetRoutes(prefixes)
			}
		})
	}
}

// BenchmarkInitialSync: a downstream router connects and receives the full
// table (time includes the peer decoding every UPDATE).
func BenchmarkInitialSync(b *testing.B) {
	for _, n := range benchSizes {
		prefixes := benchPrefixes(n, 1)
		want := snapLen(prefixes)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			srv := benchServer(b)
			srv.SetRoutes(prefixes)
			b.ReportAllocs()
			var p *benchPeer
			i := 0
			for b.Loop() {
				p = pipeSession(b, srv, i)
				p.waitFor(b, want, 0)
				i++
			}
			b.ReportMetric(float64(p.msgs.Load()), "updates")
			b.ReportMetric(float64(p.bytes.Load())/1e6, "wireMB")
		})
	}
}

// BenchmarkChurn1pct: a feed sync replaces 1% of the list.
func BenchmarkChurn1pct(b *testing.B) {
	for _, n := range benchSizes {
		a := benchPrefixes(n, 1)
		c := append(append([]string(nil), a[:n-n/100]...), benchPrefixes(n/100, 2)...)
		perFlip := symDiff(buildSnapshot(a), buildSnapshot(c))
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			srv := benchServer(b)
			p := pipeSession(b, srv, 0)
			srv.SetRoutes(a)
			p.waitFor(b, snapLen(a), 0)
			base := p.ann.Load()
			b.ReportAllocs()
			b.ResetTimer()
			flips := int64(0)
			for b.Loop() {
				if flips%2 == 0 {
					srv.SetRoutes(c)
				} else {
					srv.SetRoutes(a)
				}
				flips++
				for p.ann.Load()-base+p.wd.Load() < flips*perFlip {
					runtime.Gosched()
				}
			}
			b.ReportMetric(float64(perFlip), "changed")
		})
	}
}

// BenchmarkMemory: heap held by N fully synced downstream sessions.
// Run with -benchtime=1x.
func BenchmarkMemory(b *testing.B) {
	const n = 160_000
	prefixes := benchPrefixes(n, 1)
	want := snapLen(prefixes)
	for _, sessions := range []int{1, 50, 200} {
		b.Run(fmt.Sprintf("%dk/%dsess", n/1000, sessions), func(b *testing.B) {
			for b.Loop() {
				srv := benchServer(b)
				var m0, m1 runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&m0)
				srv.SetRoutes(prefixes)
				peers := make([]*benchPeer, sessions)
				for i := range peers {
					peers[i] = pipeSession(b, srv, i)
				}
				for _, p := range peers {
					p.waitFor(b, want, 0)
				}
				runtime.GC()
				runtime.ReadMemStats(&m1)
				total := float64(m1.HeapAlloc) - float64(m0.HeapAlloc)
				b.ReportMetric(total/1e6, "heapMB")
				b.ReportMetric(total/float64(sessions)/1e3, "KB/sess")
				runtime.KeepAlive(srv)
			}
		})
	}
}
