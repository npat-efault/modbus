// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

//go:generate stringer -type=FnCode,ExCode -output strings.go

package modbus

import "errors"

// Modbus ADU and PDU sizes (bytes)
const (
	MaxADU    = 260
	MaxSerADU = 256
	MaxTcpADU = 260
	MaxPDU    = 253
	SerHeadSz = 1
	SerCRCSz  = 2
	TcpHeadSz = 7
)

const (
	// ExcFlag is the Exception Flag. It is set on the function
	// code to indicate an exception (error) response.
	ExcFlag byte = 1 << 7
)

// Modbus Function (request and response) codes
type FnCode byte

const (
	RdInputs      FnCode = 0x02
	RdCoils       FnCode = 0x01
	WrCoil        FnCode = 0x05
	WrCoils       FnCode = 0x0f
	RdInputRegs   FnCode = 0x04
	RdHoldingRegs FnCode = 0x03
	WrReg         FnCode = 0x06
	WrRegs        FnCode = 0x10
	MskWrReg      FnCode = 0x16
	RdWrRegs      FnCode = 0x17
	RdFIFO        FnCode = 0x18
	RdFileRec     FnCode = 0x14
	WrFileRec     FnCode = 0x15
	RdExcStatus   FnCode = 0x07
	Diag          FnCode = 0x08
	GetComCnt     FnCode = 0x0b
	GetComLog     FnCode = 0x0c
	SrvId         FnCode = 0x11
	RdDevId       FnCode = 0x2b
)

// Modbus exception codes. Used as the second field of exception
// (error) responses.
type ExCode uint8

const (
	BadFnCode  ExCode = 0x01
	BadAddress ExCode = 0x02
	BadValue   ExCode = 0x03
	SrvFail    ExCode = 0x04
	SrvAck     ExCode = 0x05
	SrvBusy    ExCode = 0x06
	ErrParity  ExCode = 0x08
	GwPathNA   ExCode = 0x0a
	GwRespFail ExCode = 0x0b
)

var (
	errFnCode  = errors.New("Invalid function code")
	errPack    = errors.New("Packing error")
	errUnpack  = errors.New("Unpacking error")
	errFnUnsup = errors.New("Function code unsuported")
)

// ReqRes is a modbus request, or a modbus response. This interface is
// implemented by all the [Req|Res|ReqRes]XXX types (structures).
type ReqRes interface {
	fmbReqRes() // Dummy. Flag type as modbus request-or-response
	Pack([]byte) ([]byte, error)
	Unpack([]byte) ([]byte, error)
	FnCode() FnCode
}

// Req is a modbus request. This interface is implemented by all the
// ReqXXX and ReqResXXX types (structures).
type Req interface {
	fmbReq() // Dummy. Flag type as modbus request
	Pack([]byte) ([]byte, error)
	Unpack([]byte) ([]byte, error)
	FnCode() FnCode
}

// Req is a modbus response. This interface is implemented by all the
// ResXXX and ReqResXXX types (structures).
type Res interface {
	fmbRes() // Dummy. Flag type as modbus response
	Pack([]byte) ([]byte, error)
	Unpack([]byte) ([]byte, error)
	FnCode() FnCode
}

// NewReq returns a Req (request) interface-value with a concrete type
// corresponding to the given ModBus function code (i.e. a pointer to
// the appropriate ReqXXX structure). If an invalid (or unsupported)
// function-code is given, it returns nil and an error.
func NewReq(f FnCode) (Req, error) {
	switch f {
	case RdInputs:
		return &ReqRdInputs{Coils: false}, nil
	case RdCoils:
		return &ReqRdInputs{Coils: true}, nil
	case WrCoil:
		return &ReqResWrCoil{}, nil
	case WrCoils:
		return nil, errFnUnsup
	case RdInputRegs:
		return &ReqRdRegs{Holding: false}, nil
	case RdHoldingRegs:
		return &ReqRdRegs{Holding: true}, nil
	case WrReg:
		return &ReqResWrReg{}, nil
	case WrRegs:
		return nil, errFnUnsup
	case MskWrReg:
		return nil, errFnUnsup
	case RdWrRegs:
		return nil, errFnUnsup
	case RdFIFO, RdFileRec, WrFileRec:
		return nil, errFnUnsup
	case RdExcStatus, Diag, GetComCnt, GetComLog:
		return nil, errFnUnsup
	case SrvId, RdDevId:
		return nil, errFnUnsup
	default:
		return nil, errFnCode
	}
}

// NewRes returns a Res (respoonse) interface-value with a concrete
// type corresponding to the given ModBus function code (i.e. a
// pointer to the appropriate ResXXX structure). If an invalid (or
// unsupported) function-code is given, it returns nil and an error.
func NewRes(f FnCode) (Res, error) {
	switch f {
	case RdInputs:
		return &ResRdInputs{Coils: false}, nil
	case RdCoils:
		return &ResRdInputs{Coils: true}, nil
	case WrCoil:
		return &ReqResWrCoil{}, nil
	case WrCoils:
		return nil, errFnUnsup
	case RdInputRegs:
		return &ResRdRegs{Holding: false}, nil
	case RdHoldingRegs:
		return &ResRdRegs{Holding: true}, nil
	case WrReg:
		return &ReqResWrReg{}, nil
	case WrRegs:
		return nil, errFnUnsup
	case MskWrReg:
		return nil, errFnUnsup
	case RdWrRegs:
		return nil, errFnUnsup
	case RdFIFO, RdFileRec, WrFileRec:
		return nil, errFnUnsup
	case RdExcStatus, Diag, GetComCnt, GetComLog:
		return nil, errFnUnsup
	case SrvId, RdDevId:
		return nil, errFnUnsup
	default:
		return nil, errFnCode
	}
}

// PDU is a byte-slice holding a ModBus PDU
type PDU []byte

func (p PDU) IsExc() bool    { return p[0]&ExcFlag != 0 }
func (p PDU) ExCode() ExCode { return ExCode(p[1]) }
func (p PDU) FnCode() FnCode { return FnCode(p[0] & ^ExcFlag) }
