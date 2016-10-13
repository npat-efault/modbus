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

// SerSlave is a modbus-over-serial slave (server). Before starting
// it, of after changing the parameters below, you *must* call the
// Init method. Parameters must not be changed while the server is
// running (i.e. after calling Start and before it returns).
type SerSlave struct {
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

	conn   DeadlineReadWriter
	rcv    SerReceiver
	synced bool
	cnt    counters
	reqBuf [MaxSerADU]byte
	resBuf [MaxSerADU]byte
}

// Init initializes the ModBus slave. Argument conn is the
// DeadlineReadWriter where the slave will receive requests and
// transmit responses to. You must call Init before starting the
// server. You must not call it again while the server is running
// (i.e. after calling method Start and before it returns).
func (ss *SerSlave) Init(conn DeadlineReadWriter) {
	// Fixup params
	if ss.Timeout <= 0 {
		ss.Timeout = DflSerSlvTimeout
	}
	if ss.FrameTimeout <= 0 {
		// Calc from baudrate ??
		ss.FrameTimeout = DflSerSlvFrameTimeout
	}
	if ss.Delay <= 0 {
		ss.Delay = DflSerDelay
	}
	if ss.SyncDelay <= 0 {
		ss.SyncDelay = DflSerSlvSyncDelay
	}
	if ss.SyncWaitMax <= 0 {
		ss.SyncWaitMax = DflSerSyncWaitMax
	}

	ss.conn = conn
	if ss.Ascii {
		// ...
	} else {
		// Create and configure receiver
		rcv := NewSerReceiverRTU(ss.conn)
		rcv.FrameTimeout = ss.FrameTimeout
		rcv.SyncDelay = ss.SyncDelay
		rcv.SyncWaitMax = ss.SyncWaitMax
		ss.rcv = rcv
	}
	ss.cnt.Init(SlvCntNum)
}

// Start starts the slave. The slave is considered running after
// calling this method, and before it returns. It is typical to
// execute this method in a separate goroutine. To stop a running
// slave close the DeadlineReadWriter you supplied at Init, and wait
// for Start to return.
func (ss *SerSlave) Start() error {
	return ss.run()
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
	// Observe delay
	if ss.Delay > 0 {
		time.Sleep(ss.Delay)
	}
	// Put a timeout on write, just in case
	// deadline := TODO(npat)
	_, err := ss.conn.Write(res)
	if err != nil {
		return wErrIO(err)
	}
	return nil
}

func (ss *SerSlave) run() error {
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
		reqADU, err = ss.rcv.ReceiveReq(reqADU, time.Time{})
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
