package bgpserver

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// BGP message types (RFC 4271 section 4.1).
const (
	msgOpen         = 1
	msgUpdate       = 2
	msgNotification = 3
	msgKeepalive    = 4
	msgRouteRefresh = 5
)

// BGP path attribute type codes.
const (
	attrOrigin          = 1
	attrASPath          = 2
	attrNextHop         = 3
	attrMED             = 4
	attrLocalPref       = 5
	attrAtomicAggregate = 6
	attrAggregator      = 7
	attrCommunities     = 8
	attrMPReachNLRI     = 14
	attrMPUnreachNLRI   = 15
	attrLargeCommunity  = 32
	attrAS4Path         = 17
	attrAS4Aggregator   = 18
)

// BGP capability codes.
const (
	capMultiProtocol   = 1
	capRouteRefresh    = 2
	capExtendedNextHop = 5
	capAS4             = 65
)

// Address families.
const (
	afiIPv4     = 1
	afiIPv6     = 2
	safiUnicast = 1
)

// Notification codes.
const (
	notifMessageHeader    = 1
	notifOpenMessage      = 2
	notifUpdateMessage    = 3
	notifHoldTimerExpired = 4
	notifFSM              = 5
	notifCease            = 6
)

// OPEN error subcodes.
const (
	openUnsupportedVersion   = 1
	openBadPeerAS            = 2
	openBadBGPPeerID         = 3
	openUnsupportedOptParam  = 4
	openUnacceptableHoldTime = 6
)

const (
	bgpVersion = 4
	asTrans    = 23456
	headerLen  = 19
	maxMsgLen  = 4096
)

var marker = [16]byte{
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
}

// message is a parsed BGP message body.
type message struct {
	Type int
	Body []byte
}

func readMessage(r io.Reader) (*message, error) {
	var hdr [headerLen]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	if hdr[0] != 0xff || hdr[1] != 0xff || hdr[2] != 0xff || hdr[3] != 0xff ||
		hdr[4] != 0xff || hdr[5] != 0xff || hdr[6] != 0xff || hdr[7] != 0xff ||
		hdr[8] != 0xff || hdr[9] != 0xff || hdr[10] != 0xff || hdr[11] != 0xff ||
		hdr[12] != 0xff || hdr[13] != 0xff || hdr[14] != 0xff || hdr[15] != 0xff {
		return nil, errors.New("bgp: invalid marker")
	}
	length := int(binary.BigEndian.Uint16(hdr[16:18]))
	if length < headerLen || length > maxMsgLen {
		return nil, fmt.Errorf("bgp: invalid message length %d", length)
	}
	kind := int(hdr[18])
	body := make([]byte, length-headerLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return &message{Type: kind, Body: body}, nil
}

func encodeMessage(kind int, body []byte) []byte {
	buf := make([]byte, headerLen+len(body))
	copy(buf[:16], marker[:])
	binary.BigEndian.PutUint16(buf[16:18], uint16(headerLen+len(body)))
	buf[18] = byte(kind)
	copy(buf[headerLen:], body)
	return buf
}

// ---------------------------------------------------------------------------
// OPEN
// ---------------------------------------------------------------------------

type capability struct {
	Code uint8
	Data []byte
}

type openMessage struct {
	Version      uint8
	MyAS         uint16
	HoldTime     uint16
	Identifier   uint32
	Capabilities []capability

	AS4          uint32
	HasAS4       bool
	IPv4Unicast  bool
	IPv6Unicast  bool
	RouteRefresh bool
}

func decodeOpen(body []byte) (*openMessage, error) {
	if len(body) < 10 {
		return nil, errors.New("bgp: short OPEN")
	}
	o := &openMessage{
		Version:    body[0],
		MyAS:       binary.BigEndian.Uint16(body[1:3]),
		HoldTime:   binary.BigEndian.Uint16(body[3:5]),
		Identifier: binary.BigEndian.Uint32(body[5:9]),
	}
	optLen := int(body[9])
	if len(body) < 10+optLen {
		return nil, errors.New("bgp: truncated OPEN optional parameters")
	}
	opts := body[10 : 10+optLen]
	for len(opts) > 0 {
		if len(opts) < 2 {
			return nil, errors.New("bgp: truncated optional parameter")
		}
		ptype := opts[0]
		plen := int(opts[1])
		if len(opts) < 2+plen {
			return nil, errors.New("bgp: truncated optional parameter value")
		}
		val := opts[2 : 2+plen]
		opts = opts[2+plen:]
		if ptype != 2 {
			continue
		}
		caps, err := decodeCapabilities(val)
		if err != nil {
			return nil, err
		}
		o.Capabilities = append(o.Capabilities, caps...)
	}
	for _, c := range o.Capabilities {
		switch c.Code {
		case capAS4:
			if len(c.Data) == 4 {
				o.AS4 = binary.BigEndian.Uint32(c.Data)
				o.HasAS4 = true
			}
		case capMultiProtocol:
			if len(c.Data) >= 4 {
				afi := binary.BigEndian.Uint16(c.Data[0:2])
				safi := c.Data[3]
				if safi == safiUnicast {
					switch afi {
					case afiIPv4:
						o.IPv4Unicast = true
					case afiIPv6:
						o.IPv6Unicast = true
					}
				}
			}
		case capRouteRefresh:
			o.RouteRefresh = true
		}
	}
	return o, nil
}

func decodeCapabilities(data []byte) ([]capability, error) {
	var out []capability
	for len(data) > 0 {
		if len(data) < 2 {
			return nil, errors.New("bgp: truncated capability")
		}
		code := data[0]
		clen := int(data[1])
		if len(data) < 2+clen {
			return nil, errors.New("bgp: truncated capability value")
		}
		out = append(out, capability{Code: code, Data: data[2 : 2+clen]})
		data = data[2+clen:]
	}
	return out, nil
}

func encodeOpen(localASN uint32, holdTime uint16, routerID uint32, ipv6 bool) []byte {
	myAS := localASN
	if myAS > 0xffff {
		myAS = asTrans
	}
	var caps []capability

	// 4-byte ASN capability is always advertised.
	as4 := make([]byte, 4)
	binary.BigEndian.PutUint32(as4, localASN)
	caps = append(caps, capability{Code: capAS4, Data: as4})

	// Multiprotocol capability for IPv4 unicast (optional but harmless).
	v4 := make([]byte, 4)
	binary.BigEndian.PutUint16(v4[0:2], afiIPv4)
	v4[2] = 0
	v4[3] = safiUnicast
	caps = append(caps, capability{Code: capMultiProtocol, Data: v4})

	if ipv6 {
		v6 := make([]byte, 4)
		binary.BigEndian.PutUint16(v6[0:2], afiIPv6)
		v6[2] = 0
		v6[3] = safiUnicast
		caps = append(caps, capability{Code: capMultiProtocol, Data: v6})
	}

	// Route refresh has no payload.
	caps = append(caps, capability{Code: capRouteRefresh, Data: nil})

	var capData []byte
	for _, c := range caps {
		capData = append(capData, c.Code, byte(len(c.Data)))
		capData = append(capData, c.Data...)
	}

	body := make([]byte, 0, 10+2+len(capData))
	body = append(body, bgpVersion)
	body = append(body, byte(myAS>>8), byte(myAS))
	body = append(body, byte(holdTime>>8), byte(holdTime))
	body = append(body, byte(routerID>>24), byte(routerID>>16), byte(routerID>>8), byte(routerID))
	if len(capData) == 0 {
		body = append(body, 0)
	} else {
		// One capability optional parameter: type(1) + length(1) + value.
		body = append(body, byte(2+len(capData)))
		body = append(body, 2, byte(len(capData)))
		body = append(body, capData...)
	}
	return encodeMessage(msgOpen, body)
}

// ---------------------------------------------------------------------------
// KEEPALIVE / NOTIFICATION
// ---------------------------------------------------------------------------

func encodeKeepalive() []byte {
	return encodeMessage(msgKeepalive, nil)
}

func encodeNotification(code, subcode uint8, data []byte) []byte {
	body := append([]byte{code, subcode}, data...)
	return encodeMessage(msgNotification, body)
}

// ---------------------------------------------------------------------------
// UPDATE encoding
// ---------------------------------------------------------------------------

type pathAttr struct {
	Flags uint8
	Code  uint8
	Value []byte
}

func encodePathAttrs(attrs []pathAttr) []byte {
	var out []byte
	for _, a := range attrs {
		out = append(out, a.Flags, a.Code)
		if len(a.Value) > 255 {
			// Extended length.
			out[len(out)-2] |= 0x10
			out = append(out, byte(len(a.Value)>>8), byte(len(a.Value)))
		} else {
			out = append(out, byte(len(a.Value)))
		}
		out = append(out, a.Value...)
	}
	return out
}

func attrOriginIGP() pathAttr {
	return pathAttr{Flags: 0x40, Code: attrOrigin, Value: []byte{0}}
}

func makeASPath(localASN uint32, as4 bool) pathAttr {
	var v []byte
	if as4 {
		v = append(v, 2, 1) // AS_SEQUENCE, one AS
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], localASN)
		v = append(v, b[:]...)
	} else {
		asn := localASN
		if asn > 0xffff {
			asn = asTrans
		}
		v = append(v, 2, 1)
		v = append(v, byte(asn>>8), byte(asn))
	}
	return pathAttr{Flags: 0x40, Code: attrASPath, Value: v}
}

func attrNextHopV4(ip [4]byte) pathAttr {
	return pathAttr{Flags: 0x40, Code: attrNextHop, Value: ip[:]}
}

func makeCommunities(comms []uint32) pathAttr {
	v := make([]byte, 0, len(comms)*4)
	for _, c := range comms {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], c)
		v = append(v, b[:]...)
	}
	return pathAttr{Flags: 0xc0, Code: attrCommunities, Value: v}
}

func attrLargeCommunities(comms []LargeCommunity) pathAttr {
	v := make([]byte, 0, len(comms)*12)
	for _, c := range comms {
		var b [12]byte
		binary.BigEndian.PutUint32(b[0:4], c.Global)
		binary.BigEndian.PutUint32(b[4:8], c.Local1)
		binary.BigEndian.PutUint32(b[8:12], c.Local2)
		v = append(v, b[:]...)
	}
	return pathAttr{Flags: 0xc0, Code: attrLargeCommunity, Value: v}
}

func attrMPReachV6(nextHop [16]byte, nlri []byte) pathAttr {
	v := make([]byte, 0, 4+1+16+1+len(nlri))
	var afi [2]byte
	binary.BigEndian.PutUint16(afi[:], afiIPv6)
	v = append(v, afi[0], afi[1], safiUnicast, 16)
	v = append(v, nextHop[:]...)
	v = append(v, 0) // reserved
	v = append(v, nlri...)
	return pathAttr{Flags: 0x80, Code: attrMPReachNLRI, Value: v}
}

func attrMPUnreachV6(nlri []byte) pathAttr {
	v := make([]byte, 0, 3+len(nlri))
	var afi [2]byte
	binary.BigEndian.PutUint16(afi[:], afiIPv6)
	v = append(v, afi[0], afi[1], safiUnicast)
	v = append(v, nlri...)
	return pathAttr{Flags: 0x80, Code: attrMPUnreachNLRI, Value: v}
}

// encodeNLRI serializes a prefix in BGP NLRI wire format.
func encodeNLRI(prefix []byte, bits int) []byte {
	out := make([]byte, 1+(bits+7)/8)
	out[0] = byte(bits)
	copy(out[1:], prefix[:(bits+7)/8])
	return out
}

func encodeUpdateV4(prefixes [][4]byte, bits []int, localASN uint32, as4 bool, nextHop [4]byte, comms []uint32, large []LargeCommunity) []byte {
	attrs := []pathAttr{attrOriginIGP(), makeASPath(localASN, as4), attrNextHopV4(nextHop)}
	if len(comms) > 0 {
		attrs = append(attrs, makeCommunities(comms))
	}
	if len(large) > 0 {
		attrs = append(attrs, attrLargeCommunities(large))
	}
	attrBytes := encodePathAttrs(attrs)

	var nlri []byte
	for i, p := range prefixes {
		nlri = append(nlri, encodeNLRI(p[:], bits[i])...)
	}

	body := make([]byte, 0, 2+2+len(attrBytes)+len(nlri))
	body = append(body, 0, 0) // withdrawn routes length
	body = append(body, byte(len(attrBytes)>>8), byte(len(attrBytes)))
	body = append(body, attrBytes...)
	body = append(body, nlri...)
	return encodeMessage(msgUpdate, body)
}

func encodeUpdateV6(prefixes [][16]byte, bits []int, localASN uint32, as4 bool, nextHop [16]byte, comms []uint32, large []LargeCommunity) []byte {
	attrs := []pathAttr{attrOriginIGP(), makeASPath(localASN, as4)}
	if len(comms) > 0 {
		attrs = append(attrs, makeCommunities(comms))
	}
	if len(large) > 0 {
		attrs = append(attrs, attrLargeCommunities(large))
	}
	var nlri []byte
	for i, p := range prefixes {
		nlri = append(nlri, encodeNLRI(p[:], bits[i])...)
	}
	attrs = append(attrs, attrMPReachV6(nextHop, nlri))
	attrBytes := encodePathAttrs(attrs)

	body := make([]byte, 0, 2+2+len(attrBytes))
	body = append(body, 0, 0)
	body = append(body, byte(len(attrBytes)>>8), byte(len(attrBytes)))
	body = append(body, attrBytes...)
	return encodeMessage(msgUpdate, body)
}

// ---------------------------------------------------------------------------
// UPDATE decoding (minimum needed to validate and observe peer input)
// ---------------------------------------------------------------------------

type updateMessage struct {
	Withdrawn        []netipPrefix
	Announced        []netipPrefix
	ASPathLen        int
	Communities      []uint32
	LargeCommunities []LargeCommunity
	MPAnnounced      []netipPrefix
	MPWithdrawn      []netipPrefix
}

type netipPrefix struct {
	Addr [16]byte
	Bits int
	IsV6 bool
}

func decodeUpdate(body []byte) (*updateMessage, error) {
	u := &updateMessage{}
	if len(body) < 4 {
		return nil, errors.New("bgp: short UPDATE")
	}
	wlen := int(binary.BigEndian.Uint16(body[0:2]))
	if len(body) < 2+wlen+2 {
		return nil, errors.New("bgp: truncated withdrawn routes")
	}
	withdrawn := body[2 : 2+wlen]
	rest := body[2+wlen:]
	alen := int(binary.BigEndian.Uint16(rest[0:2]))
	if len(rest) < 2+alen {
		return nil, errors.New("bgp: truncated path attributes")
	}
	attrBytes := rest[2 : 2+alen]
	nlri := rest[2+alen:]

	prefixes, err := decodeNLRI(withdrawn, false)
	if err != nil {
		return nil, err
	}
	u.Withdrawn = prefixes

	announced, err := decodeNLRI(nlri, false)
	if err != nil {
		return nil, err
	}
	u.Announced = announced

	for len(attrBytes) > 0 {
		if len(attrBytes) < 3 {
			return nil, errors.New("bgp: truncated path attribute")
		}
		flags := attrBytes[0]
		code := attrBytes[1]
		alen2 := int(attrBytes[2])
		off := 3
		if flags&0x10 != 0 {
			if len(attrBytes) < 4 {
				return nil, errors.New("bgp: truncated extended attribute")
			}
			alen2 = int(binary.BigEndian.Uint16(attrBytes[2:4]))
			off = 4
		}
		if len(attrBytes) < off+alen2 {
			return nil, errors.New("bgp: truncated attribute value")
		}
		val := attrBytes[off : off+alen2]
		attrBytes = attrBytes[off+alen2:]

		switch code {
		case attrASPath:
			u.ASPathLen = asPathCount(val)
		case attrCommunities:
			for i := 0; i+4 <= len(val); i += 4 {
				u.Communities = append(u.Communities, binary.BigEndian.Uint32(val[i:i+4]))
			}
		case attrLargeCommunity:
			for i := 0; i+12 <= len(val); i += 12 {
				u.LargeCommunities = append(u.LargeCommunities, LargeCommunity{
					Global: binary.BigEndian.Uint32(val[i : i+4]),
					Local1: binary.BigEndian.Uint32(val[i+4 : i+8]),
					Local2: binary.BigEndian.Uint32(val[i+8 : i+12]),
				})
			}
		case attrMPReachNLRI:
			if len(val) < 5 {
				return nil, errors.New("bgp: short MP_REACH")
			}
			afi := binary.BigEndian.Uint16(val[0:2])
			nhLen := int(val[3])
			if len(val) < 4+nhLen+1 {
				return nil, errors.New("bgp: truncated MP_REACH next hop")
			}
			rest := val[4+nhLen+1:]
			ps, err := decodeNLRI(rest, afi == afiIPv6)
			if err != nil {
				return nil, err
			}
			u.MPAnnounced = append(u.MPAnnounced, ps...)
		case attrMPUnreachNLRI:
			if len(val) < 3 {
				return nil, errors.New("bgp: short MP_UNREACH")
			}
			afi := binary.BigEndian.Uint16(val[0:2])
			ps, err := decodeNLRI(val[3:], afi == afiIPv6)
			if err != nil {
				return nil, err
			}
			u.MPWithdrawn = append(u.MPWithdrawn, ps...)
		}
	}
	return u, nil
}

func decodeNLRI(data []byte, isV6 bool) ([]netipPrefix, error) {
	var out []netipPrefix
	for len(data) > 0 {
		bits := int(data[0])
		byteLen := (bits + 7) / 8
		if len(data) < 1+byteLen {
			return nil, errors.New("bgp: truncated NLRI")
		}
		p := netipPrefix{Bits: bits, IsV6: isV6}
		copy(p.Addr[:], data[1:1+byteLen])
		out = append(out, p)
		data = data[1+byteLen:]
	}
	return out, nil
}

func asPathCount(val []byte) int {
	count := 0
	for len(val) > 0 {
		if len(val) < 2 {
			return count
		}
		segType := val[0]
		segLen := int(val[1])
		if segType == 2 { // AS_SEQUENCE
			count += segLen
		}
		// Assume 2-byte ASNs for observation purposes.
		skip := 2 + segLen*2
		if segType == 1 { // AS_SET
			skip = 2 + segLen*2
		}
		if len(val) < skip {
			return count
		}
		val = val[skip:]
	}
	return count
}
