package chrony

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	// "github.com/getsentry/raven-go"

	"github.com/getsentry/sentry-go"
	"github.com/netdata/go-orchestrator/module"
)

const (
	// protoVersionNumber is the protocol version for this client
	protoVersionNumber = uint8(6)
	// pktTypeCMDRequest is the request packet type
	pktTypeCMDRequest = uint8(1)
	// pktTypeCMDReply is the reply packet type
	pktTypeCMDReply = uint8(2)
	// reqTracking identifies a tracking request
	reqTracking = uint16(33)
	// rpyTracking identifies a tracking reply
	rpyTracking = uint16(5)
	// floatExpBits represent 32-bit floating-point format consisting of 7-bit signed exponent
	floatExpBits = 7
	// floatCoefBits represent chronyFloat 25-bit signed coefficient without hidden bit
	floatCoefBits = 25
	// precision scaling factor
	scaleFactor = 1000000000
)

var (
	// chronyCmdAddr is the chrony local port
	chronyCmdAddr = "127.0.0.1:323"
)

func init() {
	// raven.SetDSN("https://a646445445cc43168fd66a4095c17526:9df6ff045c7440f196900e573d13903c@sentry.i.agoralab.co/23")
	creator := module.Creator{
		Defaults: module.Defaults{
			Disabled: true,
		},
		Create: func() module.Module { return New() },
	}

	module.Register("chrony", creator)
}

// New creates chronyCollector exposing local status of a chrony daemon
func New() *chronyCollector {
	raddr, _ := net.ResolveUDPAddr("udp", chronyCmdAddr)
	return &chronyCollector{
		metrics: make(map[string]int64),
		cmdAddr: raddr,
	}
}

// Cleanup makes cleanup
func (c *chronyCollector) Cleanup() {
}

// Init makes initialization
func (c *chronyCollector) Init() bool {
	err := sentry.Init(sentry.ClientOptions{
		Dsn: "https://a646445445cc43168fd66a4095c17526:9df6ff045c7440f196900e573d13903c@sentry.i.agoralab.co/23",
	})

	if err != nil {
		c.Errorf("Sentry initialization failed: %v\n", err)
	}
	// sentry.Flush(time.Second * 5)
	return true
}

// Check makes check
func (c *chronyCollector) Check() bool {
	if c.cmdAddr == nil {
		c.Errorf("invalid chrony cmdAddr %s", chronyCmdAddr)
		return false
	}
	// TODO need something real to check udp socket
	conn, err := net.DialUDP("udp", nil, c.cmdAddr)
	if err != nil {
		c.Errorf("unable connect to chrony addr %s, is chrony up and running?", c.cmdAddr)
		return false
	}

	// TODO: need protocol version check
	defer conn.Close()
	c.Debugf("connecting to chrony addr %s", conn.LocalAddr())
	return true
}

// Charts creates Charts dynamically (each gpu device will be a family)
func (c *chronyCollector) Charts() *Charts {
	return charts.Copy()
}

// Collect collects metrics
func (c *chronyCollector) Collect() map[string]int64 {
	tracking, err := fetchTracking(c.cmdAddr)
	if err != nil {
		c.Errorf("fetch tracking status failed: %s", err)
		sentry.CaptureException((FetchingChronyError)(err.Error()))
		c.metrics["running"] = 0
		return c.metrics
	}
	c.Debugf("RefID: %s, Stratum: %s, RefTime: %s, CurrentCorrection: %s, FreqPpm: %s, SkewPpm: %s, RootDelay: %s, RootDispersion: %s, LeapStatus: %s, LastUpdateInterval: %s, LastOffset: %s, CurrentCorrection: %s", tracking.RefID, tracking.Stratum, tracking.RefTime.EpochSeconds(), tracking.CurrentCorrection.Float64(), tracking.FreqPpm.Float64(), tracking.SkewPpm.Float64(), tracking.RootDelay.Float64(), tracking.RootDispersion.Float64(), tracking.LeapStatus, tracking.LastUpdateInterval.Float64(), tracking.LastOffset.Float64(), tracking.CurrentCorrection.Float64(), tracking.RefTime)

	c.metrics["running"] = 1
	c.metrics["stratum"] = (int64)(tracking.Stratum)
	c.metrics["leap_status"] = (int64)(tracking.LeapStatus)
	c.metrics["root_delay"] = (int64)(tracking.RootDelay.Int64())
	c.metrics["root_dispersion"] = (int64)(tracking.RootDispersion.Int64())
	c.metrics["skew"] = (int64)(tracking.SkewPpm.Int64())
	c.metrics["frequency"] = (int64)(tracking.FreqPpm.Int64())
	c.metrics["last_offset"] = (int64)(tracking.LastOffset.Int64())
	c.metrics["rms_offset"] = (int64)(tracking.RmsOffset.Int64())
	c.metrics["update_interval"] = (int64)(tracking.LastUpdateInterval.Int64())
	c.metrics["current_correction"] = (int64)(tracking.LastUpdateInterval.Int64())

	// report root dispersion error to sentry when error > 100ms
	rd := tracking.RootDispersion.Float64()
	if rd > 0.1 {
		c.Debugf("sending sentry error for RootDispersionTooLargeError: %g", rd)
		// raven.CaptureError((RootDispersionTooLargeError)(rd), map[string]string{"service": "chrony"})
		sentry.CaptureException((RootDispersionTooLargeError)(rd))
	}

	// report frequency change to sentry when step > 500ppm
	fp := tracking.FreqPpm.Float64()
	if fp > 500 {
		c.Debugf("sending sentry error for FreqChangeTooFastError: %g", fp)
		sentry.CaptureException((FreqChangeTooFastError)(fp))
	}

	if tracking.LeapStatus > 0 {
		c.Debugf("sending sentry error for LeapStatusError: %g", tracking.LeapStatus)
		sentry.CaptureException((LeapStatusError)(tracking.LeapStatus))
	}

	return c.metrics
}

func fetchTracking(addr *net.UDPAddr) (*trackingPayload, error) {
	var attempt uint16
	var seqNumber uint32

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := trackingReq(conn, attempt, seqNumber); err != nil {
		return nil, err
	}

	payload, err := trackingReply(conn, attempt, seqNumber)
	if err != nil {
		return nil, err
	}
	return &payload, nil
}

// trackingReply parses a tracking reply
func trackingReq(conn *net.UDPConn, attempt uint16, seqNumber uint32) error {
	buf := new(bytes.Buffer)
	req := trackingRequest{
		ProtoVer: protoVersionNumber,
		PktType:  pktTypeCMDRequest,
		Command:  reqTracking,
		Attempt:  attempt,
	}
	if err := binary.Write(buf, binary.BigEndian, req); err != nil {
		return err
	}

	n, err := conn.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write tracking request: %s", err)
	}
	if n != buf.Len() {
		return fmt.Errorf("short write (%d)", n)
	}
	return nil
}

// trackingReply parses a tracking reply
func trackingReply(conn *net.UDPConn, attempt uint16, seqNumber uint32) (trackingPayload, error) {
	var reply replyPacket
	var payload trackingPayload

	dgram := make([]byte, 4096)
	_, err := conn.Read(dgram)
	if err != nil {
		return payload, err
	}

	rd := bytes.NewReader(dgram)
	if err := binary.Read(rd, binary.BigEndian, &reply); err != nil {
		return payload, fmt.Errorf("failed to read tracking reply: %s", err)
	}
	if reply.ProtoVer != protoVersionNumber {
		return payload, fmt.Errorf("unexpected tracking protocol version: %d", reply.ProtoVer)
	}
	if reply.PktType != pktTypeCMDReply {
		return payload, fmt.Errorf("unexpected tracking packet type: %d", reply.PktType)
	}
	// if reply.Command != rpyTracking {
	// 	return payload, fmt.Errorf("unexpected reply command: %d", reply.Command)
	// }
	if reply.SeqNum != seqNumber {
		return payload, fmt.Errorf("unexpected tracking packet seqNumber: %d", reply.SeqNum)
	}

	if err := binary.Read(rd, binary.BigEndian, &payload); err != nil {
		return payload, fmt.Errorf("failed reading tracking payload: %s", err)
	}

	return payload, nil
}
