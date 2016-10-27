// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "time"

// SerMaster is a modbus-over-serial master (client). Exported fields
// can be changed between calls to master methods. All have
// reasonable defaults.
type SerMaster struct {
	// Response timeout. Counting approx. from the *end* of the
	// request transmission, until the reception of the first
	// response byte.
	Timeout time.Duration
	// Number of request retransmission, if no response is
	// received.
	Retrans int

	rcv    SerReceiver
	trx    SerTransmitter
	synced bool
}

// NewSerMaster returns a modbus-over-serial master (client) that uses
// the given serial receiver (rcv) and transmitter (trx).
func NewSerMaster(rcv SerReceiver, trx SerTransmitter) *SerMaster {
	return &SerMaster{rcv: rcv, trx: trx}
}

// SerMasterConf are the modbus-over-serial master (client)
// configuration parameters used by function NewSerMasterStd. Zero
// values for all fields will be replaced by reasonable defaults.
type SerMasterConf struct {
	// Serial bus bitrate. Used for timeout calculations
	Baudrate int
	// Response timeout. Counting approx. from the *end* of the
	// request transmission, until the reception of the first
	// response byte.
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
	// frame. (RTU only)
	SyncDelay time.Duration
	// Time to wait to (re-)synchronize before giving up. (RTU
	// only)
	SyncWaitMax time.Duration
	// Use ASCII frame encoding
	Ascii bool
}

// NewSerMasterStd returns a modbus-over-serial master (client) that
// uses the standard receiver (SerReceiver{RTU|ASCII}) and transmitter
// (SerTransmitter{RTU|ASCII}). The master receives and transmits
// frames on conn, and is configured using the parameters in cfg.
func NewSerMasterStd(conn DeadlineReadWriter, cfg SerMasterConf) *SerMaster {
	var sm *SerMaster
	// Fixup params
	if cfg.Baudrate <= 0 {
		cfg.Baudrate = 9600
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DflSerMstTimeout
	}
	if cfg.FrameTimeout <= 0 {
		// Calc from baudrate ??
		cfg.FrameTimeout = DflSerMstFrameTimeout
	}
	if cfg.Delay <= 0 {
		cfg.Delay = DflSerDelay
	}
	if cfg.SyncDelay <= 0 {
		cfg.SyncDelay = DflSerMstSyncDelay
	}
	if cfg.SyncWaitMax <= 0 {
		cfg.SyncWaitMax = DflSerSyncWaitMax
	}
	if cfg.Ascii {
		panic("TODO(npat): ASCII encoding not supported!")
	} else {
		// Create and configure receiver
		rcv := NewSerReceiverRTU(conn)
		rcv.FrameTimeout = cfg.FrameTimeout
		rcv.SyncDelay = cfg.SyncDelay
		rcv.SyncWaitMax = cfg.SyncWaitMax
		sm.rcv = rcv
		// Create and configure transmitter
		trx := NewSerTransmitterRTU(conn)
		trx.Baudrate = cfg.Baudrate
		trx.Delay = cfg.Delay
		// Create and configure master
		sm = NewSerMaster(rcv, trx)
		sm.Timeout = cfg.Timeout
		sm.Retrans = cfg.Retrans
	}
	return sm
}

// SndRcv transmits the request ADU and receives a response ADU. The
// response ADU is appended to byte-slice b. It is ok for b to be
// nil. Pass a non-nil b if you want to use pre-allocated space for
// the response. Returns the appended-to byte-slice as a SerADU. On
// error it returns b unaffected, along with the error. Exception
// responses by the slave are not considered errors.
//
// Errors returned by SndRcv are: ErrFrame (framing error, cannot
// receive response frame), ErrCRC (bad response frame CRC),
// ErrTimeout (response reception timeout), ErrSync (failed to sync to
// the bus), and any I/O error returned by the DeadlineReadWriter,
// wrapped in ErrIO. Of these ErrIO, and possibly ErrSync should be
// considered fatal.
func (sm *SerMaster) SndRcv(req SerADU, b []byte) (SerADU, error) {
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
		var deadline time.Time
		deadline, err = sm.trx.Transmit(req)
		if err != nil {
			sm.synced = false
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
				sm.synced = false
				break
			}
			// Timeout. Retry.
			continue
		}
		// Response ok
		b = a
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
