package chrony

import (
	"fmt"
	"math"
	"net"

	"github.com/netdata/go-orchestrator/module"
)

// chronyCollector is the main collector for chrony
type chronyCollector struct {
	module.Base // should be embedded by every module
	metrics     map[string]int64
	// cmdAddr stores the IP address for chrony command service
	cmdAddr *net.UDPAddr
	// trackingStratum is the stratum metrics
	// trackingStratum string = "local chrony stratum."
}

// trackingRequest holds a tracking request
type trackingRequest struct {
	ProtoVer  uint8
	PktType   uint8
	Res1      uint8
	Res2      uint8
	Command   uint16
	Attempt   uint16
	SeqNumber uint32
	Pad       [92]byte
}

// trackingPayload is the payload for tracking replies (`rpyTracking`)
type trackingPayload struct {
	RefID              uint32
	IPAddrHigh         uint64
	IPAddrLow          uint64
	IPFamily           uint16
	Pad1               uint16
	Stratum            uint16
	LeapStatus         uint16
	RefTime            chronyTimespec
	CurrentCorrection  chronyFloat
	LastOffset         chronyFloat
	RmsOffset          chronyFloat
	FreqPpm            chronyFloat
	ResidFreqPpm       chronyFloat
	SkewPpm            chronyFloat
	RootDelay          chronyFloat
	RootDispersion     chronyFloat
	LastUpdateInterval chronyFloat
}

// replyPacket is the common header for all replies
type replyPacket struct {
	ProtoVer uint8
	PktType  uint8
	Res1     uint8
	Res2     uint8
	Command  uint16
	Reply    uint16
	Status   uint16
	Pad1     uint16
	Pad2     uint16
	Pad3     uint16
	SeqNum   uint32
	Pad4     uint32
	Pad5     uint32
}

// chronyFloat is the custom chrony timespec type (`Timespec`)
type chronyTimespec struct {
	TvSecHigh uint32
	TvSecLow  uint32
	TvNSec    uint32
}

// EpochSeconds returns the number of seconds since epoch
func (ct chronyTimespec) EpochSeconds() float64 {
	ts := uint64(ct.TvSecHigh) << 32
	ts += uint64(ct.TvSecLow)
	return float64(ts)
}

/* 32-bit floating-point format consisting of 7-bit signed exponent
   and 25-bit signed coefficient without hidden bit.
   The result is calculated as: 2^(exp - 25) * coef */
type chronyFloat int32

// ToFloat does magic to decode float from int32.
// Code is copied and translated to Go from original C sources.
func (f chronyFloat) Float64() float64 {
	var exp, coef int32

	x := uint32(f)

	exp = int32(x >> floatCoefBits)
	if exp >= 1<<(floatExpBits-1) {
		exp -= 1 << floatExpBits
	}
	exp -= floatCoefBits

	coef = int32(x % (1 << floatCoefBits))
	if coef >= 1<<(floatCoefBits-1) {
		coef -= 1 << floatCoefBits
	}

	return float64(coef) * math.Pow(2.0, float64(exp))
}

// Int64 returns the 64bits float value
func (cf chronyFloat) Int64() int64 {
	return int64(cf.Float64() * scaleFactor)
}

// RootDispersionTooLargeError
type RootDispersionTooLargeError float64

func (f RootDispersionTooLargeError) Error() string {
	return fmt.Sprintf("root dispersion too large: %g", float64(f))
}

// FreqChangeTooFastError
type FreqChangeTooFastError float64

func (f FreqChangeTooFastError) Error() string {
	return fmt.Sprintf("chrony frequency change too fast: %g", float64(f))
}

// LeapStatusError
type LeapStatusError float64

func (f LeapStatusError) Error() string {
	return fmt.Sprintf("chrony abnormal leap status: %g", float64(f))
}

// FetchingChronyError
type FetchingChronyError string

func (f FetchingChronyError) Error() string {
	return fmt.Sprintf("can't read from chrony socket: %s", string(f))
}
