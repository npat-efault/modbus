// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "time"

// SerMaster is a modbus-over-serial master (client). Before using, or
// after changing any of the fields below, you *must* call the Init
// method.
type SerMaster struct {
	conn DeadlineReadWriter
	// Response reception timeout (counting from start of the
	// request transmission)
	Timeout time.Duration
	// Delay between the reception of a response and the
	// transmission of the next request.
	Delay time.Duration
	// Number of request retransmission, if no response is
	// received.
	Retrans int
	// Time the bus has to remain idle before the master is
	// considered synchronized. Applies only after the receiver
	// detects a frame error or a bad frame.
	SyncDelay time.Duration
	// Time to wait to (re-)synchronize before giving up.
	SyncWaitMax time.Duration
	// Use ASCII frame encoding
	Ascii bool

	rcv      SerReceiver
	timeLast time.Time
}

// Init must be called before using the master, and after changing any
// of it's config params.
func (sm *SerMaster) Init(conn DeadlineReadWriter) {
	sm.conn = conn
	// Fixup params
	if sm.Timeout <= 0 {
		sm.Timeout = DflSerMstTimeout
	}
	if sm.Delay <= 0 {
		sm.Delay = DflSerDelay
	}
	if sm.SyncDelay <= 0 {
		sm.SyncDelay = DflSerSyncDelay
	}
	if sm.SyncWaitMax <= 0 {
		sm.SyncWaitMax = DflSerSyncWaitMax
	}
	if sm.Ascii {
		// sc.rcv = &SerReceiverASCII{r: sc.Conn}
	} else {
		// Create and configure receiver
		rcv := NewSerReceiverRTU(true, sm.conn)
		rcv.Timeout = sm.Timeout
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
// responses by the server (slave) are not considered errors.
//
// Errors returned by SndRcv are: ErrComm (cannot receive response),
// or any error returned by the DeadlineReadWriter, wrapped in ErrIO.
func (sm *SerMaster) SndRcv(req SerADU, b []byte) (SerADU, error) {
	// Observe delay
	if !sm.Ascii {
		dt := time.Now().Sub(sm.timeLast)
		if dt < sm.Delay {
			time.Sleep(sm.Delay - dt)
		}
	}
	var err error
	var try int
	for try = sm.Retrans + 1; try > 0; try-- {
		deadline := time.Now().Add(sm.Timeout)

		// Transmit request. Put a deadline on transmission
		// just in case. It should never expire.
		err = sm.conn.SetWriteDeadline(deadline)
		if err != nil {
			err = wErrIO(err)
			break
		}
		_, err = sm.conn.Write(req)
		if err != nil {
			// Don't retransmit on timeout.
			// We should not tmo on write.
			err = wErrIO(err)
		}

		if req.Node() == 0x0 {
			// Broadcast, no response
			break
		}

		// Receive response
		var a SerADU
		a, err = sm.rcv.Receive(b)
		if err != nil {
			if err == ErrSync {
				err = ErrComm
				break
			}
			if _, ok := err.(*ErrIO); ok {
				break
			}
			// Retry
			continue
		}
		// Response ok
		b = a
		sm.timeLast = time.Now()
		break
	}
	if try == 0 {
		err = ErrComm
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
