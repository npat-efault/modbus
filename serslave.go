// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "time"

type SerHandler interface {
	Handle(node uint8, req Req) Res
}

type SerHandlerRaw interface {
	Handle(req SerADU, res SerADU) SerADU
}

// SerSlave is a modbus-over-serial slave (server). Exported fields
// can be changed between calls to master methods. All have reasonable
// defaults.
type SerSlave struct {
	// Node-Id this slave responds to. If zero, all request are
	// passed to the handler, which decides to process and respond
	// to them or not.
	NodeId uint8
	// Handler and HandelrRaw is where requests (with matching
	// node-ids) are passed to. With both handlers nil the slave
	// only monitors the bus for requests from the master and
	// responses from other slaves. It never responds to a
	// request. With both handlers non-nil, Handler is used.
	Handler    SerHandler
	HandlerRaw SerHandlerRaw

	rcv    SerReceiver
	trx    SerTransmitter
	synced bool
	cnt    counters
}

// NewSerSlave returns a modbus-over-serial slave (server) that uses
// the given serial receiver (rcv) and transmitter (trx).
func NewSerSlave(rcv SerReceiver, trx SerTransmitter) {
	return &SerSlave{rcv: rcv, trx: trx}
}

// SerSlaveConf are the modbus-over-serial slave (server)
// configuration parameters used by function NewSerSlaveStd. Zero
// values for all fields will be replaced by reasonable defaults.
type SerSlaveConf struct {
	// Node-Id this slave responds to. If zero, all request are
	// passed to the handler, which decides to process them or
	// not.
	NodeId uint8
	// Handler and HandelrRaw is where requests (with matching
	// node-ids) are passed to. With both handlers nil the slave
	// only monitors the bus for requests from the master and
	// responses from other slaves. It never responds to a
	// request. With both handlers non-nil, Handler is used.
	Handler    SerHandler
	HandlerRaw SerHandlerRaw
	// Response timeout. Counting approx. from the *end* of the
	// request transmission, until the reception of the first
	// response byte.
	Timeout time.Duration
	// Frame timeout. Maximum time allowed for nothing to be
	// received, while the reception of a request or response has
	// started.
	FrameTimeout time.Duration
	// Delay between the reception of a request and the
	// transmission of the response.
	Delay time.Duration
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

// NewSerSlaveStd returns a modbus-over-serial slave (server) that
// uses the standard receiver (SerReceiver{RTU|ASCII}) and transmitter
// (SerTransmitter{RTU|ASCII}). The slave receives and transmits
// frames on conn, and is configured using the parameters in cfg.
func NewSerSlaveStd(conn DeadlineReadWriter, cfg SerSlaveConf) *SerSlave {
	var ss *SerSlave
	// Fixup params
	if cfg.Timeout <= 0 {
		cfg.Timeout = DflSerSlvTimeout
	}
	if cfg.FrameTimeout <= 0 {
		// Calc from baudrate ??
		cfg.FrameTimeout = DflSerSlvFrameTimeout
	}
	if cfg.Delay <= 0 {
		cfg.Delay = DflSerDelay
	}
	if cfg.SyncDelay <= 0 {
		cfg.SyncDelay = DflSerSlvSyncDelay
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
		ss.rcv = rcv
		// Create and configure transmitter
		trx := NewSerTransmitterRTU(conn)
		trx.Baudrate = cfg.Baudrate
		trx.Delay = cfg.Delay
		// Create and configure slave
		ss = NewSerSlave(rcv, trx)
		ss.NodeId = cfg.NodeId
		ss.SerHandler = cfg.SerHandler
		ss.SerHandlerRaw = cfg.SerHandlerRaw
		ss.Timeout = cfg.Timeout
	}
	ss.cnt.Init(SlvCntNum)
}

func (ss *SerSlave) handle(reqADU SerADU) SerADU {
	resADU := SerADU(ss.resBuf[:])
	if ss.Handler == nil {
		if ss.HandlerRaw != nil {
			return ss.HandlerRaw.Handle(reqADU, resADU)
		}
		return nil
	}
	node, fn := reqADU.Node(), reqADU.FnCode()
	exc := ResExc{Function: fn}

	req, err := NewReq(fn)
	if err != nil {
		exc.ExCode = BadFnCode
		resADU, _ = SerPack(resADU, node, &exc)
		return resADU
	}
	_, err = req.Unpack(reqADU)
	if err != nil {
		exc.ExCode = BadValue
		resADU, _ = SerPack(resADU, node, &exc)
		return resADU
	}
	res := ss.Handler.Handle(node, req)
	if res == nil {
		return nil
	}
	resADU, err = SerPack(resADU, node, res)
	if err != nil {
		exc.ExCode = SrvFail
		resADU, _ = SerPack(resADU, node, &exc)
		return resADU
	}
	return resADU
}

// TODO(npat): Add echo-mode support?

func (ss *SerSlave) transmit(res SerADU) error {
	// Transmit timeout, just in case. Should never expire.
	const txTmo = 2 * time.Second
	// Observe delay
	if ss.Delay > 0 {
		time.Sleep(ss.Delay)
	}
	ss.conn.SetWriteDeadline(time.Now().Add(txTmo))
	_, err := ss.conn.Write(res)
	if err != nil {
		return wErrIO(err)
	}
	return nil
}

// Start starts the slave. The slave is considered running after
// calling this method, and until it returns. It is typical to execute
// this method in a separate goroutine. To stop a running slave close
// the DeadlineReadWriter used by the transmitter and receiver, and
// wait for Start to return.
func (ss *SerSlave) Start() error {
	// Wait for request timeout. Go back waiting if it expires.
	const reqTmo = 1 * time.Second
	var err error
	for {
		if !ss.synced {
			err = ss.rcv.Sync()
			if err != nil {
				break
			}
			ss.synced = true
		}
		reqADU := SerADU(ss.reqBuf[:])
		// Receive request
		reqADU, err = ss.rcv.ReceiveReq(reqADU, time.Now().Add(reqTmo))
		if err != nil {
			if _, ok := err.(*ErrIO); ok {
				break
			}
			if err == ErrFrame || err == ErrCRC {
				ss.synced = false
			}
			// Timeout (??) next request
			continue
		}
		if reqADU.Node() == 0x00 {
			// Boadcast, ignore response
			_ = ss.handle(reqADU)
			// Next request
			continue
		}
		if ss.NodeId == 0 || ss.NodeId == reqADU.Node() {
			// Ours, probably
			resADU := ss.handle(reqADU)
			if resADU != nil {
				// Ours, transmit response
				err = ss.transmit(resADU)
				if err != nil {
					break
				}
				// Next request
				continue
			}
		}
		// Not ours, receive response
		resADU := SerADU(ss.resBuf[:])
		deadline := time.Now().Add(ss.Timeout)
		resADU, err = ss.rcv.ReceiveRes(resADU, deadline)
		if err != nil {
			if _, ok := err.(*ErrIO); ok {
				break
			}
			if err == ErrFrame || err == ErrCRC {
				ss.synced = false
			}
			// Timeout, next request
			continue
		}
	}
	return err
}

// Counter returns the counter indicated by argument cnt. See
// SlvCntXXX constants for suppported counters. It is ok to call this
// method while the slave is running.
func (ss *SerSlave) Counter(cnt Counter) uint64 {
	return ss.cnt.Get(cnt)
}

// Counters returns all slave counters. Each array slot is a
// counter. See SlvCntXXX constants for supported counters.
func (ss *SerSlave) Counters(cnt Counter) []uint64 {
	return ss.cnt.GetAll()
}
