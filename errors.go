// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

// ErrIO is used to wrap I/O errors.
type ErrIO struct {
	// Err is the original error
	Err error
}

func (e *ErrIO) Error() string { return "I/O error (" + e.Err.Error() + ")" }

func (e *ErrIO) Timeout() bool {
	type timeout interface {
		Timeout() bool
	}
	t, ok := e.Err.(timeout)
	return ok && t.Timeout()
}

func (e *ErrIO) Temporary() bool {
	type temporary interface {
		Temporary() bool
	}
	t, ok := e.Err.(temporary)
	return ok && t.Temporary()
}

// wrap e in ErrIO
func wErrIO(e error) error {
	if _, ok := e.(*ErrIO); ok {
		return e
	}
	return &ErrIO{e}
}

// errT is an error-type with tags
type errT struct {
	tags uint
	msg  string
}

// tags for errT-typed errors
const (
	efTmp = 1 << iota
	efTmo
	efCom
)

// mkErr returns a new errT error with the given tags (or'ed together)
// and message.
func mkErr(tags uint, msg string) error {
	return &errT{tags: tags, msg: msg}
}

// mkErr returns a new errT error with the given message (no tags).
func newErr(msg string) error {
	return mkErr(0, msg)
}

func (e *errT) Error() string { return e.msg }

func (e *errT) Timeout() bool { return e.tags&efTmo != 0 }

func (e *errT) Temporary() bool { return e.tags&efTmp != 0 }

func (e *errT) Comm() bool { return e.tags&efCom != 0 }

// IsTimeout, tests if error is a timeout
func IsTimeout(e error) bool {
	type tmoError interface {
		Timeout() bool
	}
	if et, ok := e.(tmoError); ok {
		return et.Timeout()
	}
	return false
}

// IsTemporary, tests if error is temporary
func IsTemporary(e error) bool {
	type tmoError interface {
		Timeout() bool
	}
	if et, ok := e.(tmoError); ok {
		return et.Timeout()
	}
	return false
}

// IsComm, tests if error is a communication error
func IsComm(e error) bool {
	type commError interface {
		Comm() bool
	}
	if ec, ok := e.(commError); ok {
		return ec.Comm()
	}
	return false
}

// Errors returned by functions and methods in this package. Other
// errors may as well be returned which are not exported and subject
// to change. Also, errors by functions and methods in other packages
// may be returned. Consult the documentation of specific function or
// method for details.
//
// Errors tagged with "efTmp", "efTmo", and "efCom" test true with
// IsTemporary(), IsTimeout(), and IsComm() respectivelly.
var (
	errFnCode  = newErr("Invalid function code")
	errFnUnsup = newErr("Function code unsuported")
	errPack    = newErr("Packing error")
	errUnpack  = newErr("Unpacking error")
	errTODO    = newErr("TODO(npat) Unspecified error")

	// Serial ADU receiver errors
	ErrFrame   = mkErr(efCom, "Frame reception error")
	ErrCRC     = mkErr(efCom, "Bad frame CRC")
	ErrTimeout = mkErr(efCom|efTmo|efTmp, "Frame reception time-out")
	ErrSync    = newErr("Failed to synchronize")

	// Errors returned by the serial master
	ErrRequest  = newErr("Bad or invalid request")
	ErrResponse = newErr("Bad or invalid response")
)
