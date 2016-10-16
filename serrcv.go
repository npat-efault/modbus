// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import (
	"bufio"
	"io"
	"time"
)

// ModBus over serial default timing parameters
const (
	// Conservative default values

	// For masters
	DflSerMstTimeout      = 150 * time.Millisecond
	DflSerMstFrameTimeout = 60 * time.Millisecond
	DflSerMstSyncDelay    = DflSerMstFrameTimeout
	// For slaves
	DflSerSlvTimeout      = 100 * time.Millisecond
	DflSerSlvFrameTimeout = 40 * time.Millisecond
	DflSerSlvSyncDelay    = DflSerSlvTimeout

	// Common
	DflSerDelay       = 10 * time.Millisecond
	DflSerSyncWaitMax = 10 * time.Second
	minTimeout        = 50 * time.Millisecond
)

// DeadlineReadWriter is an io.ReadWriter with additional methods to
// set deadlines on read and write calls. Network connections
// (net.Conn) implement this interfacce, so do poller fd's
// (gitbub.com/npat-efault/poller).
type DeadlineReadWriter interface {
	DeadlineReader
	DeadlineWriter
}

// DeadlineReader is an io.Reader with an additional method to
// set timeouts on read calls.
type DeadlineReader interface {
	io.Reader
	SetReadDeadline(t time.Time) error
}

// DeadlineWriter is an io.Writer with an additional method to
// set timeouts on write calls.
type DeadlineWriter interface {
	io.Writer
	SetWriteDeadline(t time.Time) error
}

// sizer calculates the size of modbus-serial ADUs. A new sizer (or
// one initialized to zero) must be used for each ADU.
type sizer struct {
	sz int
}

// sizeRes returns the remaining bytes for the patially received
// response frame in b. If the frame-size cannot be determined
// (unsupported function code), it returns 0, false
func (s *sizer) sizeRes(b []byte) (remain int, ok bool) {
	if s.sz != 0 {
		return s.sz - len(b), true
	}
	if len(b) < 5 {
		return 5 - len(b), true
	}
	if b[1]&ExcFlag != 0 {
		s.sz = 3 + SerCRCSz
		return s.sz - len(b), true
	}
	switch FnCode(b[1]) {
	case RdCoils, RdInputs, RdHoldingRegs, RdInputRegs, RdWrRegs,
		RdFileRec, WrFileRec, GetComLog, SlaveId:
		s.sz = int(b[2]) + 3 + SerCRCSz
		return s.sz - len(b), true
	case WrCoil, WrReg, WrCoils, WrRegs, GetComCnt:
		s.sz = 6 + SerCRCSz
		return s.sz - len(b), true
	case MskWrReg:
		s.sz = 8 + SerCRCSz
		return s.sz - len(b), true
	case RdExcStatus:
		s.sz = 3 + SerCRCSz
		return s.sz - len(b), true
	case RdFIFO:
		s.sz = (int(b[2])<<8 | int(b[3])) + 3 + SerCRCSz
		return s.sz - len(b), true
	default:
		return 0, false
	}
}

// sizeReq returns the remaining bytes for the patially received
// request frame in b. If the frame-size cannot be determined
// (unsupported function code), it returns 0, false
func (s *sizer) sizeReq(b []byte) (remain int, ok bool) {
	if s.sz != 0 {
		return s.sz - len(b), true
	}
	if len(b) < 2 {
		return 2 - len(b), true
	}
	switch FnCode(b[1]) {
	case RdCoils, RdInputs, RdHoldingRegs, RdInputRegs, WrCoil, WrReg:
		s.sz = 6 + SerCRCSz
		return s.sz - len(b), true
	case RdExcStatus, GetComCnt, GetComLog, SlaveId:
		s.sz = 2 + SerCRCSz
		return s.sz - len(b), true
	case WrCoils, WrRegs:
		if len(b) < 7 {
			return 7 - len(b), true
		}
		s.sz = int(b[6]) + 7 + SerCRCSz
		return s.sz - len(b), true
	case RdFileRec, WrFileRec:
		if len(b) < 3 {
			return 3 - len(b), true
		}
		s.sz = int(b[2]) + 3 + SerCRCSz
		return s.sz - len(b), true
	case MskWrReg:
		s.sz = 8 + SerCRCSz
		return s.sz - len(b), true
	case RdWrRegs:
		if len(b) < 10 {
			return 10 - len(b), true
		}
		s.sz = int(b[10]) + 11 + SerCRCSz
		return s.sz - len(b), true
	case RdFIFO:
		s.sz = 4 + SerCRCSz
		return s.sz - len(b), true
	case RdDevId:
		s.sz = 5 + SerCRCSz
		return s.sz - len(b), true
	default:
		return 0, false
	}
}

// SerReceiver is a frame receiver for modbus-over-serial frames
// (ADUs). There are two implementations: One for RTU-encoded ADUs,
// and one for ASCII-encoded ADUs.
type SerReceiver interface {
	// ReceiveReq and ReceiveRes read (receive) response or
	// request serial frames (ADUs) respectively. They append the
	// received ADU at byte-slice b. It is ok for b to be
	// nil. Pass a non-nil b if you want to use pre-allocated
	// space. The first byte of the frame must be received before
	// the given deadline expires. Returns the appended-to
	// byte-slice as a SerADU. On error it returns b unaffected,
	// along with the error.
	//
	// The error returned can be one of the following: ErrFrame
	// (cannot receive frame), ErrCRC (bad frame CRC), ErrTimeout
	// (frame reception timed-out), or any I/O error returned
	// by the DeadlineReader, wrapped in ErrIO.
	//
	// See specific implementations for more details.
	ReceiveReq(b []byte, deadline time.Time) (SerADU, error)
	ReceiveRes(b []byte, deadline time.Time) (SerADU, error)

	// Buf returns an empty (zero-len) byte-slice positioned at
	// the beginning of the internal receiver buffer. You can pass
	// Buf() as the first (b) argument to ReceiveReq or ReceiveRes
	// and avoid the copy from the internal buffer. Buf() may
	// return nil.
	Buf() []byte

	// Sync must be called to syncronize the master or slave to
	// the serial bus. Sync returns nil (succesfully synced),
	// ErrSync (failed to sync), or any error returned by the
	// DeadlineReader wrapped in ErrIO. See specific
	// implementation for more details.
	Sync() error
}

// SerReceiverRTU is the SerFrameReceiver implementation for
// RTU-encoded ADUs. Exported fields can be changed between calls to
// receiver methods. All have reasonable defaults.
//
// For more details on timing parameters see the file
// "rtu-timing.txt", distributed with the package sources.
//
type SerReceiverRTU struct {
	// FrameTimeout is the intra-frame timeout. It is started when
	// the first frame byte is received and refreshed whith the
	// reception of any subsequent frame-bytes.
	FrameTimeout time.Duration
	// Duration the line should remain idle in order to consider
	// the receiver re-synchronized.
	SyncDelay time.Duration
	// Maximum time to wait for re-synchronization, before
	// giving-up and returning ErrSync.
	SyncWaitMax time.Duration
	r           DeadlineReader
	buf         [MaxSerADU]byte
}

// NewSerReceiverRTU returns a new receiver for RTU-encoded ADUs.
func NewSerReceiverRTU(r DeadlineReader) *SerReceiverRTU {
	return &SerReceiverRTU{
		r:            r,
		FrameTimeout: DflSerMstFrameTimeout,
		SyncDelay:    DflSerMstSyncDelay,
		SyncWaitMax:  DflSerSyncWaitMax,
	}
}

// ReceiveReq receives a REQUEST ADU. Upon entry the receiver must be
// synchronized to the start of the request frame. After a successful
// frame reception, the receiver returns imediately. The caller must
// make sure that an appropriate delay is observed before transmitting
// the next response. After a frame reception failure (ErrFrame or
// ErrCRC), the caller must re-synchronize the receiver by calling the
// Sync method.
func (rcv *SerReceiverRTU) ReceiveReq(b []byte,
	deadline time.Time) (SerADU, error) {
	return rcv.receive(b, deadline, true)
}

// ReceiveRes receives a RESPONSE ADU. Upon entry the receiver must be
// synchronized to the start of the response frame. After a successful
// frame reception, the receiver returns imediately. The caller must
// make sure that an appropriate delay is observed before transmitting
// the next request. After a frame reception failure (ErrFrame or
// ErrCRC), the caller must re-synchronize the receiver by calling the
// Sync method.
func (rcv *SerReceiverRTU) ReceiveRes(b []byte,
	deadline time.Time) (SerADU, error) {
	return rcv.receive(b, deadline, false)
}

func appendBytes(a, b []byte) []byte {
	if len(b) == 0 {
		return a
	}
	if cap(a) >= len(a)+len(b) && &a[:len(a)+1][len(a)] == &b[0] {
		return a[:len(a)+len(b)]
	}
	return append(a, b...)
}

func (rcv *SerReceiverRTU) receive(b []byte,
	deadline time.Time, req bool) (SerADU, error) {

	var be = rcv.buf[:]
	var fr = be[0:0]
	var sz sizer

	rcv.r.SetReadDeadline(deadline)

	var nrem int
	if req {
		nrem, _ = sz.sizeReq(fr)
	} else {
		nrem, _ = sz.sizeRes(fr)
	}
	for {
		n, err := rcv.r.Read(be[:nrem])
		be = be[n:]
		fr = fr[:len(fr)+n]
		var ok bool
		if req {
			nrem, ok = sz.sizeReq(fr)
		} else {
			nrem, ok = sz.sizeRes(fr)
		}
		if !ok {
			// Unsuported function code
			return b, ErrFrame
		}
		if nrem == 0 {
			// Full frame received
			break
		}
		if err != nil {
			if IsTimeout(err) {
				// TODO(npat): Separate in-frame tmo?
				return b, ErrTimeout
			}
			return b, wErrIO(err)
		}
		rcv.r.SetReadDeadline(time.Now().Add(rcv.FrameTimeout))
	}
	a := SerADU(fr)
	if !a.CheckCRC() {
		return b, ErrCRC
	}
	b = appendBytes(b, a)
	return b, nil
}

func (rcv *SerReceiverRTU) Buf() []byte {
	return rcv.buf[0:0]
}

// Sync synchronizes the slave or master on the bus. Must be called
// before the first request is transmitted (master) or before the
// first frame is received (slave). Must also be called to
// resynchronize the master or slave after a frame error (ErrFrame, or
// ErrCRC).
func (rcv *SerReceiverRTU) Sync() error {
	b := make([]byte, 16)
	tend := time.Now().Add(rcv.SyncWaitMax)
	for {
		rcv.r.SetReadDeadline(time.Now().Add(rcv.SyncDelay))
		_, err := rcv.r.Read(b)
		if err != nil {
			if !IsTimeout(err) {
				return wErrIO(err)
			}
			return nil
		}
		if time.Now().After(tend) {
			return ErrSync
		}
	}
}

type SerReceiverASCII struct {
	master       bool
	r            DeadlineReader
	br           bufio.Reader
	Timeout      time.Duration
	FrameTimeout time.Duration
}
