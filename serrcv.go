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
	DflSerMstTimeout  = 100 * time.Millisecond
	DflSerDelay       = 10 * time.Millisecond
	DflSerSyncDelay   = 50 * time.Millisecond
	DflSerSyncWaitMax = 5 * time.Second
	DflSerBaudrate    = 9600
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

func isTimeout(e error) bool {
	type tmoError interface {
		Timeout() bool
	}
	if et, ok := e.(tmoError); ok {
		return et.Timeout()
	}
	return false
}

// serBusTime calculates the time it takes to transmit n bytes, at the
// given bitrate. The time calculated is multipled by the factor, and
// an appropriate deadline is returned.
func serBusTime(baudrate int, n int, factor float64) time.Time {
	t := uint64(n) * 10 * uint64(1000000) / uint64(baudrate)
	d := time.Duration(float64(t) * factor)
	if d < minTimeout {
		d = minTimeout
	}
	return time.Now().Add(d)
}

// serResFrameSz returns the size (in bytes) of the modbus-serial
// response frame (ADU), given the initial part of a partially read
// frame (in byte-slice b). If the frame-size cannot be determined yet
// (the initial part is not enough), SerResFrameSz returns 0,
// false. This function is a kludge used by masters on systems that
// cannot detect frame-boundaries using silent intervals. See
// [2],§2.5.1.1,pg.13
func serResFrameSz(b []byte) (length int, ok bool) {
	if len(b) < 3 {
		return 0, false
	}
	if b[1]&ExcFlag != 0 {
		return 3 + SerCRCSz, true
	}
	switch FnCode(b[1]) {
	case RdCoils, RdInputs, RdHoldingRegs, RdInputRegs, RdWrRegs,
		RdFileRec, WrFileRec, GetComLog, SlaveId:
		return int(b[2]) + 3 + SerCRCSz, true
	case WrCoil, WrReg, WrCoils, WrRegs, GetComCnt:
		return 6 + SerCRCSz, true
	case MskWrReg:
		return 8 + SerCRCSz, true
	case RdExcStatus:
		return 3 + SerCRCSz, true
	case RdFIFO:
		if len(b) < 4 {
			return 0, false
		}
		return (int(b[2])<<8 | int(b[3])) + 3 + SerCRCSz, true
	default:
		if SerADU(b).CheckCRC() {
			return len(b), true
		}
		return 0, false
	}
}

// TODO(npat): These cons a req or resp for every frame, and
// immediatelly drop it.

func trySerUnpackReq(a SerADU) bool {
	r, err := NewReq(a.FnCode())
	if err != nil {
		if err == errFnUnsup {
			return true
		}
		return false
	}
	x, err := r.Unpack(a.PDU())
	if err != nil || len(x) != 0 {
		return false
	}
	return true
}

func trySerUnpackRes(a SerADU) bool {
	r, err := NewRes(a.FnCode())
	if err != nil {
		if err == errFnUnsup {
			return true
		}
		return false
	}
	x, err := r.Unpack(a.PDU())
	if err != nil || len(x) != 0 {
		return false
	}
	return true
}

// SerReceiver is a frame receiver for modbus-over-serial frames
// (ADUs). There are two implementations: One for RTU-encoded ADUs,
// and one for ASCII-encoded ADUs.
type SerReceiver interface {
	// Receive reads (receives) a serial frame (ADU). It appends
	// the ADU at byte-slice b. It is ok for b to be nil. Pass a
	// non-nil b if you want to use pre-allocated space. Returns
	// the appended-to byte-slice as a SerADU. On error it returns
	// b unaffected, along with the error.
	//
	// The error returned can be one of the following: ErrFrame
	// (cannot receive frame), ErrCRC (bad frame CRC), ErrTimeout
	// (frame reception timed-out), ErrSync (cannot re-synchronize
	// after frame reception failure), or any I/O error returned
	// by the DeadlineReader, wrapped in ErrIO. Of these, only the
	// ErrIO-wrapped errors (and *possibly* ErrSync) can be
	// considered fatal.
	//
	// See specific implementations for more details.
	Receive(b []byte) (SerADU, error)

	// Sync can be used to syncronize the master or slave to the
	// serial bus. Sync returns nil (succesfully synced), ErrSync
	// (failed to sync), or any error returned by the
	// DeadlineReader wrapped in ErrIO. See specific
	// implementation for more details.
	Sync() error
}

// SerReceiverRTU is the SerFrameReceiver implementation for
// RTU-encoded ADUs. It operates it two modes master, and slave.
//
// In master mode the receiver reads (receives) only RESPONSE frames
// (ADUs). In slave mode it receives both request and response frames
// (assuming the slave is connected to a half-duplex, multi-drop bus).
type SerReceiverRTU struct {
	master bool
	r      DeadlineReader
	br     *bufio.Reader

	// You can change these parameters between calls.
	//
	// Serial bus bitrate. Used for timeout calculations.
	Baudrate int
	// In master mode: Timeout until the reception of the first
	// response ADU byte, counting from when Receive was
	// called. In slave mode: Timeout for the reception of a
	// complete frame, counting from the reception of the first
	// byte. In slave mode, autocalculated if zero.
	Timeout time.Duration
	// Duration the line should remain idle in order to consider
	// the receiver re-synchronized. Applies only when the
	// receiver has detected a frame error or bad frame.
	SyncDelay time.Duration
	// Maximum time to wait for re-synchronization, before
	// giving-up and returning ErrSync.
	SyncWaitMax time.Duration
}

// NewSerReceiverRTU returns a new master or slave receiver for
// RTU-encoded ADUs.
func NewSerReceiverRTU(master bool, r DeadlineReader) *SerReceiverRTU {
	rcv := &SerReceiverRTU{
		master:      master,
		r:           r,
		Baudrate:    DflSerBaudrate,
		Timeout:     DflSerMstTimeout,
		SyncDelay:   DflSerSyncDelay,
		SyncWaitMax: DflSerSyncWaitMax,
	}
	if !rcv.master {
		rcv.Timeout = 0
		rcv.br = bufio.NewReaderSize(r, MaxSerADU)
	}
	return rcv
}

// Receive receives an ADU. Upon entry the receiver must be
// synchronized to the start of the response frame. After a successful
// frame reception, the receiver returns imediately. The caller must
// make sure that an appropriate delay is observed before transmitting
// the next request or response. After a frame reception failure, the
// receiver is re-synchronized for the next request transmission or
// frame reception, unless ErrSync (failed to re-synchronize) is
// returned. See also the docs of the Receive method of the
// SerReceiver interface.
func (rcv *SerReceiverRTU) Receive(b []byte) (SerADU, error) {
	if rcv.master {
		return rcv.receiveMaster(b)
	} else {
		return rcv.receiveSlave(b)
	}
}

func (rcv *SerReceiverRTU) receiveMaster(b []byte) (SerADU, error) {
	var buf [MaxSerADU]byte
	var be = buf[:]
	var fr = be[0:0]
	var nr, sz int
	var ok bool

	// Set receiver deadline. Must be updated when the first byte
	// is received.
	rcv.r.SetReadDeadline(time.Now().Add(rcv.Timeout))
	updatedDeadline := false
	// Determine frame size (read "head")
	for !ok {
		if nr >= MaxSerADU {
			// Cannot determine frame size,
			// try to synchronize and abort
			err := rcv.Sync()
			if err != nil {
				return b, err
			}
			return b, ErrFrame
		}
		n, err := rcv.r.Read(be)
		nr += n
		fr = fr[:nr]
		be = be[n:]
		sz, ok = serResFrameSz(fr)
		if err != nil {
			if ok && nr >= sz {
				// Full frame and error.
				// Return frame, hide error
				a := SerADU(fr[:sz])
				if err := rcv.checkADU(a); err != nil {
					return b, err
				}
				b = append(b, a...)
				return b, nil
			}
			if isTimeout(err) {
				return b, ErrTimeout
			}
			return b, wErrIO(err)
		}
		// Update the deadline. Assume max frame.
		if !updatedDeadline {
			deadline := serBusTime(rcv.Baudrate, MaxSerADU-nr, 1.5)
			rcv.r.SetReadDeadline(deadline)
			updatedDeadline = true
		}
	}
	// Fix the deadline now that we know the size
	deadline := serBusTime(rcv.Baudrate, sz-nr, 1.5)
	rcv.r.SetReadDeadline(deadline)
	// Read rest
	for nr < sz {
		n, err := rcv.r.Read(be)
		nr += n
		be = be[n:]
		if err != nil && nr < sz {
			// TODO(npat): ErrTimeout here
			// Error before frame
			return b, wErrIO(err)
		}
	}
	// Capture frame. Anything past sz is garbage.
	a := SerADU(fr[:sz])
	if err := rcv.checkADU(a); err != nil {
		return b, err
	}
	b = append(b, a...)
	return b, nil
}

func (rcv *SerReceiverRTU) receiveSlave(b []byte) (SerADU, error) {
	var fr = SerADU(b[len(b):len(b)])
	var check bool

	// Clear initial receiver deadline
	rcv.r.SetReadDeadline(time.Time{})

	for len(fr) < MaxSerADU && !check {
		ch, err := rcv.br.ReadByte()
		if err != nil {
			if isTimeout(err) {
				return b, ErrTimeout
			}
			return b, wErrIO(err)
		}
		if len(fr) == 0 {
			// Start of possible frame, set receiver deadline
			var deadline time.Time
			if rcv.Timeout > 0 {
				deadline = time.Now().Add(rcv.Timeout)
			} else {
				deadline = serBusTime(rcv.Baudrate, MaxSerADU, 1.5)
			}
			rcv.r.SetReadDeadline(deadline)
		}
		fr = append(fr, ch)
		if len(fr) < MinSerADU {
			continue
		}
		check = fr.CheckCRC()
	}
	if !check {
		// No frame detected
		// Clear recption buffer and resync
		rcv.br.Reset(rcv.r)
		err := rcv.Sync()
		if err != nil {
			return b, err
		}
		return b, ErrFrame
	}

	if !trySerUnpackReq(fr) && !trySerUnpackRes(fr) {
		// Bad frame, cannot unpack
		// Clear recption buffer and resync
		rcv.br.Reset(rcv.r)
		err := rcv.Sync()
		if err != nil {
			return b, err
		}
		return b, ErrFrame
	}

	return append(b, fr...), nil
}

func (rcv *SerReceiverRTU) checkADU(a SerADU) error {
	if !a.CheckCRC() {
		err := rcv.Sync()
		if err != nil {
			return err
		}
		return ErrCRC
	}
	return nil
}

// Sync synchronizes the slave or master on the bus. Can, optionally,
// be called before the first request is transmitted (master mode) or
// before the first frame is received (slave mode). Can also,
// optionally, be called after Receive returns ErrSync. There is no
// need to call it between subsequent request or response
// transmissions or receptions. See also the docs of the Sync method
// of the SerReceiver interface.
func (rcv SerReceiverRTU) Sync() error {
	b := make([]byte, 16)
	tend := time.Now().Add(rcv.SyncWaitMax)
	for {
		rcv.r.SetReadDeadline(time.Now().Add(rcv.SyncDelay))
		_, err := rcv.r.Read(b)
		if err != nil {
			if !isTimeout(err) {
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
