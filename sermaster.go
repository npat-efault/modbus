// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "time"

const (
	// Minimum auto-calculated timeout
	SerMinTimeout = 50 * time.Millisecond
	// Bits per transmitted character
	SerBitsPerChar = 10
)

// SerBusTime calculates the time it takes to transmit "n" chars, at
// the given bitrate. The time calculated is multipled by "factor",
// and clamped-down by SerMinTimeout. It is returned as a timeout
// (relative) along with the respective deadline (absolute).
func SerBusTime(baudrate int, n int, factor float64) (time.Duration, time.Time) {
	ns := uint64(n) * SerBitsPerChar * uint64(1000000000) / uint64(baudrate)
	d := time.Duration(float64(ns)*factor) * time.Nanosecond
	if d < SerMinTimeout {
		d = SerMinTimeout
	}
	return d, time.Now().Add(d)
}

// SerMaster is a modbus-over-serial master (client). Before using, or
// after changing any of the parameters below, you *must* call the
// Init method.
type SerMaster struct {
	conn DeadlineReadWriter
	// Serial bus bitrate. Used for timeout calculations
	Baudrate int
	// Response timeout. Counting approx. from the *end* of the
	// request transmission, until the reception of the first
	// response byte (not dependent on baudrate)
	Timeout time.Duration
	// Frame timeout. Maximum time allowed for nothing to be
	// received, while the reception of a response has started.
	FrameTimeout time.Duration
	// Delay between the reception of a response and the
	// transmission of the next request.
	Delay time.Duration
	// Number of request retransmission, if no response is
	// received.
	Retrans int
	// Time the bus has to remain idle before the master is
	// considered synchronized. The master synchronizes on the
	// first call and after it detects a frame error or a bad
	// frame.
	SyncDelay time.Duration
	// Time to wait to (re-)synchronize before giving up.
	SyncWaitMax time.Duration
	// Use ASCII frame encoding
	Ascii bool

	rcv      SerReceiver
	timeLast time.Time
	synced   bool
}

// Init must be called before using the master, and after changing any
// of it's config params.
func (sm *SerMaster) Init(conn DeadlineReadWriter) {
	sm.conn = conn
	// Fixup params
	if sm.Baudrate <= 0 {
		sm.Baudrate = 9600
	}
	if sm.Timeout <= 0 {
		sm.Timeout = DflSerMstTimeout
	}
	if sm.FrameTimeout <= 0 {
		// Calc from baudrate ??
		sm.FrameTimeout = DflSerMstFrameTimeout
	}
	if sm.Delay <= 0 {
		sm.Delay = DflSerDelay
	}
	if sm.SyncDelay <= 0 {
		sm.SyncDelay = DflSerMstSyncDelay
	}
	if sm.SyncWaitMax <= 0 {
		sm.SyncWaitMax = DflSerSyncWaitMax
	}
	if sm.Ascii {
		// sc.rcv = &SerReceiverASCII{r: sc.Conn}
	} else {
		// Create and configure receiver
		rcv := NewSerReceiverRTU(sm.conn)
		// Don't set timeout, we use rcv.Deadline instead
		// rcv.Timeout = sm.Timeout
		rcv.FrameTimeout = sm.FrameTimeout
		rcv.SyncDelay = sm.SyncDelay
		rcv.SyncWaitMax = sm.SyncWaitMax
		sm.rcv = rcv
	}
}

// SndRcv transmits the request ADU and receives a response ADU. The
// response ADU is appended to byte-slice b. It is ok for b to be
// nil. Pass a non-nil b if you want to use pre-allocated space for
// the response. Returns the appended-to byte-slice as a SerADU. On
// error it returns b unaffected, along with the error. Exception
// responses by the slave are not considered errors.
//
// Errors returned by SndRcv are: ErrFrame (cannot receive response
// frame), ErrCRC (bad response frame CRC), ErrTimeout (response
// reception timeout), ErrSync (failed to sync to the bus), and any
// I/O error returned by the DeadlineReadWriter, wrapped in ErrIO. Of
// these ErrIO, and possibly ErrSync should be considered fatal.
func (sm *SerMaster) SndRcv(req SerADU, b []byte) (SerADU, error) {
	if sm.synced {
		// Observe delay
		if !sm.Ascii {
			dt := time.Now().Sub(sm.timeLast)
			if dt < sm.Delay {
				time.Sleep(sm.Delay - dt)
			}
		}
	}
	var err error
	for try := sm.Retrans + 1; try > 0; try-- {
		// Sync to bus, if required
		if !sm.synced {
			if err = sm.rcv.Sync(); err != nil {
				break
			}
			sm.synced = true
		}
		// Transmit request
		// Calculate request transmission time
		_, deadline := SerBusTime(sm.Baudrate, len(req), 1.0)
		sm.conn.SetWriteDeadline(deadline)
		_, err = sm.conn.Write(req)
		if err != nil {
			// Don't retransmit on timeout.
			// We should not timeout on write.
			err = wErrIO(err)
			break
		}

		if req.Node() == 0x0 {
			// Broadcast, no response
			break
		}

		// Receive response
		var a SerADU
		// Set receiver deadline (take into account the
		// request transmission time)
		deadline = deadline.Add(sm.Timeout)
		a, err = sm.rcv.ReceiveRes(b, deadline)
		if err != nil {
			if err == ErrFrame || err == ErrCRC {
				sm.synced = false
				continue
			}
			if _, ok := err.(*ErrIO); ok {
				break
			}
			// Timeout. Retry.
			continue
		}
		// Response ok
		b = a
		sm.timeLast = time.Now()
		break
	}
	return b, err
}

// Do packs and transmits request req, receives a response, and
// unpacks it in res. If res is nil, a propper response type is
// allocated. Do returns the unpacked response. On error it returns
// nil and the error. Exception responses by the server are
// considered, and returned as, errors (ResExc implements the error
// interface).
//
// Appart from exception responses from slaves, errors returned by Do
// are: ErrRequest (bad reuest), ErrResponse (bad or invalid
// response), ErrCommunication (cannot receive response), and any
// error that may be returned by the DeadlineReadWriter, wrapped in
// ErrIO.
func (sm *SerMaster) Do(node uint8, req Req, res Res) (Res, error) {
	return nil, nil
}
