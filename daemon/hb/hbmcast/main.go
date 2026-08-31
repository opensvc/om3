// Package hbmcast implement a hb multicast driver
package hbmcast

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/hbcfg"
	"github.com/opensvc/om3/v3/daemon/hb/hbaudit"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

// T is the multicast heartbeat
//
// The maximum size of messages that this heartbeat is able to send and receive depends on
// var Max... values
//
// The maximum size of hb message to send is MaxFragments * MaxChunkSize: 10M
//
// The maximum size of hb message to receive is MaxFragments * MaxDatagramSize: 12M
//
// The maximum total data size from a source is MaxMessages * MaxFragments * MaxDatagramSize: 100M
type (
	T struct {
		hbcfg.T
	}

	// fragment is one datagram's worth of a message.
	//
	// The json tags are the framing this driver used before, kept so a
	// peer on an older release goes on being understood. What is sent is
	// the binary framing below.
	fragment struct {
		MsgID string `json:"mid"`
		Chunk []byte `json:"c"`
		Index int    `json:"i"`
		Total int    `json:"n"`
	}
)

// The binary fragment framing.
//
// A datagram is a fixed header followed by the chunk, unaltered:
//
//	0    2   magic "omh"
//	3        framing version, a digit
//	4   19   message id, the 16 bytes of a uuid
//	20  21   fragment index, 1 based, big endian uint16
//	22  23   fragment count, big endian uint16
//	24  ..   chunk
//
// It replaces a json envelope carrying the chunk as a []byte, which json
// encodes as base64: a 50KB chunk travelled as 68KB of text, and the
// receiver paid a json parse and a base64 decode per datagram, before it
// could even know which message the fragment belonged to. A message is
// parsed once per fragment where it is decrypted once in total, so the
// framing, not the message, was the cost.
//
// The expansion also broke the size arithmetic. MaxChunkSize is 51200,
// which as base64 in the envelope is 68337 bytes: larger than the
// receiver's read buffer and larger than a legal UDP datagram, so any
// message over 51200 bytes had at least one fragment that could not be
// sent at all. Raw, the same chunk is 51224 bytes on the wire, and
// MaxChunkSize means what it says.
//
// The version digit is what keeps the next change from being a flag day.
const (
	fragMagic     = "omh"
	fragVersion1  = '1'
	fragHeaderLen = 24
)

// encodeFragment frames one chunk for the wire.
func encodeFragment(msgID uuid.UUID, index, total int, chunk []byte) []byte {
	dgram := make([]byte, fragHeaderLen+len(chunk))
	copy(dgram[0:3], fragMagic)
	dgram[3] = fragVersion1
	copy(dgram[4:20], msgID[:])
	binary.BigEndian.PutUint16(dgram[20:22], uint16(index))
	binary.BigEndian.PutUint16(dgram[22:24], uint16(total))
	copy(dgram[fragHeaderLen:], chunk)
	return dgram
}

// decodeFragment reads a datagram in either framing, the binary one being
// told apart by its magic. Anything else is tried as the json framing
// that preceded it.
//
// The chunk is copied. The caller reads into one buffer for the life of
// the receiver and holds fragments in the assembly map until the message
// is complete, so a chunk pointing into that buffer would be overwritten
// by the next datagram.
func decodeFragment(b []byte) (fragment, error) {
	if len(b) >= fragHeaderLen && string(b[0:3]) == fragMagic {
		if b[3] != fragVersion1 {
			return fragment{}, fmt.Errorf("unsupported fragment framing version %q", b[3])
		}
		chunk := make([]byte, len(b)-fragHeaderLen)
		copy(chunk, b[fragHeaderLen:])
		return fragment{
			MsgID: string(b[4:20]),
			Index: int(binary.BigEndian.Uint16(b[20:22])),
			Total: int(binary.BigEndian.Uint16(b[22:24])),
			Chunk: chunk,
		}, nil
	}
	var f fragment
	if err := json.Unmarshal(b, &f); err != nil {
		return fragment{}, err
	}
	return f, nil
}

var (
	// TODO define common rule for message length, hb ucast defines msgMaxSize 10*1000*1000)

	// MaxMessages is the maximum number of messages from a source
	MaxMessages = 10

	// MaxFragments is the maximum number of fragments when Tx split the message
	// into fragments.
	MaxFragments = 200

	// MaxChunkSize is the maximum size of chunk in a fragment
	MaxChunkSize = 50 * 1024

	// MaxDatagramSize is the maximum size of a datagram to read. It is
	// the largest a UDP payload can legally be, so that whatever reaches
	// the socket is read whole: a datagram longer than the buffer is
	// silently truncated, and a truncated fragment corrupts its message.
	// The binary framing needs 51224 of it; the rest is headroom for a
	// peer still sending the json framing, whose datagrams are a third
	// larger.
	MaxDatagramSize = 65507
)

func New() hbcfg.Confer {
	t := &T{}
	var i interface{} = t
	return i.(hbcfg.Confer)
}

func init() {
	hbcfg.Register("multicast", New)
}

// Configure implements the Configure function of Confer interface for T
func (t *T) Configure(ctx context.Context) {
	log := plog.NewDefaultLogger().Attr("ctx", "daemon/hb/hbmcast").Attr("hb_name", t.Name()).WithPrefix("daemon: hb: mcast: " + t.Name() + ": configure: ")
	hbaudit.AttachActiveAuditIfAny(ctx, log, "hb", "hb.main", strings.Replace(t.Name(), "hb#", "hb:", 1))
	interval := t.GetDuration("interval", 5*time.Second)
	timeout := t.GetDuration("timeout", 15*time.Second)
	intf := t.GetString("intf")
	port := t.GetInt("port")
	addr := t.GetString("addr")
	nodes := t.GetStrings("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetStrings(k)
	}
	oNodes := hostname.OtherNodes(nodes)
	log.Tracef("timeout=%s interval= %s port=%d nodes=%s onodes=%s", timeout, interval,
		port, nodes, oNodes)
	t.SetNodes(oNodes)
	t.SetInterval(interval)
	t.SetTimeout(timeout)
	signature := fmt.Sprintf("type: hb.mcast, port: %d nodes: %s timeout: %s intf: %s interval: %s",
		port, nodes, timeout, intf, interval)
	t.SetSignature(signature)
	log.Debugf("signature: [%s]", signature)
	name := t.Name()

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		log.Errorf("resolve udp addr: %s", err)
		return
	}

	var ifi *net.Interface
	var laddr *net.UDPAddr

	if intf != "" {
		ifi, err = net.InterfaceByName(intf)
		if err != nil {
			log.Errorf("can't get interface by name: %s", err)
			return
		}
		log.Tracef("set rx interface %s", ifi.Name)

		addrs, err := ifi.Addrs()
		if err != nil {
			log.Warnf("intf %s addrs: %s", ifi.Name, err)
			return
		}
		for _, addr := range addrs {
			addrStr := addr.String()
			l := strings.Split(addrStr, "/")
			laddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", l[0], 0))
			if err != nil {
				log.Tracef("intf %s make tx laddr from addr %s: %s", ifi.Name, addr, err)
			} else {
				break
			}
		}
		log.Tracef("set tx interface %s laddr %s", ifi.Name, laddr)
	}

	tx := newTx(ctx, name, oNodes, laddr, udpAddr, timeout, interval)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, udpAddr, ifi, timeout)
	t.SetRx(rx)
}
