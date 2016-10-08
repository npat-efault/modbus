// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "errors"

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

// tmoErrT is an error-type that tests true with isTimeout /
// isTemporary
type tmoErrT struct {
	msg string
}

func (e *tmoErrT) Error() string {
	return e.msg
}

func (e *tmoErrT) Timeout() bool { return true }

func (e *tmoErrT) Temporary() bool { return true }

func tmoErr(msg string) error {
	return &tmoErrT{msg}
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

func isTemporary(e error) bool {
	type tmoError interface {
		Timeout() bool
	}
	if et, ok := e.(tmoError); ok {
		return et.Timeout()
	}
	return false
}

// commErrT is an error-type that tests true with isComm
type commErrT struct {
	msg string
}

func (e *commErrT) Error() string {
	return e.msg
}

func (e *commErrT) Comm() bool { return true }

func commErr(msg string) error {
	return &commErrT{msg}
}

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
// may be returned. Consult the documentation of each specific
// function or method for details.
//
// Errors marked with "CM" test true with IsComm().
var (
	errFnCode  = errors.New("Invalid function code")
	errFnUnsup = errors.New("Function code unsuported")
	errPack    = errors.New("Packing error")
	errUnpack  = errors.New("Unpacking error")
	errTODO    = errors.New("TODO(npat) Unspecified error")

	// Serial ADU receiver errors
	ErrFrame   = errors.New("Frame reception error") // CM
	ErrCRC     = errors.New("Bad frame CRC")         // CM
	ErrTimeout = tmoErr("Frame reception time-out")  // CM, TO, TMP
	ErrSync    = errors.New("Failed to synchronize")

	// Errors returned by the client
	ErrRequest  = errors.New("Bad or invalid request")
	ErrResponse = errors.New("Bad or invalid response")
)
