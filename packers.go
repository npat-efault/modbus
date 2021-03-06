// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "fmt"

// pack 16-bit words, big endian
func pU16s(b []byte, ws ...uint16) []byte {
	for _, w := range ws {
		b = append(b, byte(w>>8), byte(w))
	}
	return b
}

// unpack 16-bit words, big endian
func uU16s(b []byte, ps ...*uint16) []byte {
	for _, p := range ps {
		*p = uint16(b[0])<<8 | uint16(b[1])
		b = b[2:]
	}
	return b
}

// ResExc is the exception (error) response. Used by slaves to reply
// to bad requests. See [1],§7,pg.48. ResExc implements the error
// interface (it can be returned as an error).
type ResExc struct {
	mbRes
	Function FnCode
	ExCode   ExCode
}

func (r *ResExc) Error() string {
	return fmt.Sprintf("Modbus exception [%s:%s]", r.Function, r.ExCode)
}

func (r *ResExc) FnCode() FnCode { return r.Function }

func (r *ResExc) Pack(b []byte) ([]byte, error) {
	b = append(b, byte(r.Function)|ExcFlag, byte(r.ExCode))
	return b, nil
}

func (r *ResExc) Unpack(b []byte) ([]byte, error) {
	if len(b) < 2 {
		return b, errUnpack
	}
	r.Function = FnCode(b[0] & ^ExcFlag)
	r.ExCode = ExCode(b[1])
	return b[2:], nil
}

// ReqRdInputs is the read-coils (if .Coils == True) or
// read-discrete-inputs (if .Coils == False) request. See
// [1],§6.1,pg.12 and [1],§6.2,pg.13
type ReqRdInputs struct {
	mbReq
	Coils bool
	Addr  uint16
	Num   uint16
}

func (r *ReqRdInputs) FnCode() FnCode {
	if r.Coils {
		return RdCoils
	} else {
		return RdInputs
	}
}

func (r *ReqRdInputs) Pack(b []byte) ([]byte, error) {
	if r.Num < 1 || r.Num > 2000 {
		return b, errPack
	}
	if r.Coils {
		b = append(b, byte(RdCoils))
	} else {
		b = append(b, byte(RdInputs))
	}
	b = pU16s(b, r.Addr, r.Num)
	return b, nil
}

func (r *ReqRdInputs) Unpack(b []byte) ([]byte, error) {
	if len(b) < 5 {
		return b, errUnpack
	}
	switch FnCode(b[0]) {
	case RdInputs:
		r.Coils = false
	case RdCoils:
		r.Coils = true
	default:
		return b, errUnpack
	}
	b = uU16s(b[1:], &r.Addr, &r.Num)
	return b, nil
}

// ResRdInputs is the read-coils (if .Coils == True) or
// read-discrete-inputs (if .Coils == False) response. See
// [1],§6.1,pg.12 and [1],§6.2,pg.13
type ResRdInputs struct {
	mbRes
	Coils   bool
	BitStat []uint8
}

func (r *ResRdInputs) FnCode() FnCode {
	if r.Coils {
		return RdCoils
	} else {
		return RdInputs
	}
}

// Status returns the status of coil n (zero-based)
func (r ResRdInputs) Status(n int) bool {
	return r.BitStat[n>>3]&(1<<(uint(n)&7)) != 0
}

func (r *ResRdInputs) Pack(b []byte) ([]byte, error) {
	n := len(r.BitStat)
	if n < 1 || n > 250 {
		return b, errPack
	}
	if r.Coils {
		b = append(b, byte(RdCoils), byte(n))
	} else {
		b = append(b, byte(RdInputs), byte(n))
	}
	b = append(b, r.BitStat...)
	return b, nil
}

func (r *ResRdInputs) Unpack(b []byte) ([]byte, error) {
	if len(b) < 3 {
		return b, errUnpack
	}
	switch FnCode(b[0]) {
	case RdInputs:
		r.Coils = false
	case RdCoils:
		r.Coils = true
	default:
		return b, errUnpack
	}
	n := b[1]
	if n < 1 || n > 250 {
		return b, errUnpack
	}
	r.BitStat = r.BitStat[0:0]
	r.BitStat = append(r.BitStat, b[2:2+n]...)
	return b[2+n:], nil
}

// ReqRdRegs is the read-holding-registers (if .Holding == True) or
// read-input-registers (if .Holdings == False) request. See
// [1],§6.3,pg.15 and [1],§6.4,pg.16
type ReqRdRegs struct {
	mbReq
	Holding bool
	Addr    uint16
	Num     uint16
}

func (r *ReqRdRegs) FnCode() FnCode {
	if r.Holding {
		return RdHoldingRegs
	} else {
		return RdInputRegs
	}
}

func (r *ReqRdRegs) Pack(b []byte) ([]byte, error) {
	if r.Num < 1 || r.Num > 125 {
		return b, errPack
	}
	if r.Holding {
		b = append(b, byte(RdHoldingRegs))
	} else {
		b = append(b, byte(RdInputRegs))
	}
	b = pU16s(b, r.Addr, r.Num)
	return b, nil
}

func (r *ReqRdRegs) Unpack(b []byte) ([]byte, error) {
	if len(b) < 5 {
		return b, errUnpack
	}
	switch FnCode(b[0]) {
	case RdHoldingRegs:
		r.Holding = true
	case RdInputRegs:
		r.Holding = false
	default:
		return b, errUnpack
	}
	b = uU16s(b[1:], &r.Addr, &r.Num)
	return b, nil
}

// ResRdRegs is the read-holding-registers (if .Holding == True) or
// read-input-registers (if .Holdings == False) response. See
// [1],§6.3,pg.15 and [1],§6.4,pg.16
type ResRdRegs struct {
	mbRes
	Holding bool
	Val     []uint16
}

func (r *ResRdRegs) FnCode() FnCode {
	if r.Holding {
		return RdHoldingRegs
	} else {
		return RdInputRegs
	}
}

func (r *ResRdRegs) Pack(b []byte) ([]byte, error) {
	n := len(r.Val)
	if n < 1 || n > 125 {
		return b, errPack
	}
	if r.Holding {
		b = append(b, byte(RdHoldingRegs))
	} else {
		b = append(b, byte(RdInputRegs))
	}
	b = append(b, byte(n*2))
	b = pU16s(b, r.Val...)
	return b, nil
}

func (r *ResRdRegs) Unpack(b []byte) ([]byte, error) {
	if len(b) < 3 {
		return b, errUnpack
	}
	switch FnCode(b[0]) {
	case RdHoldingRegs:
		r.Holding = true
	case RdInputRegs:
		r.Holding = false
	default:
		return b, errUnpack
	}
	n := int(b[1])
	if n < 2 || n > 250 || n&1 != 0 {
		return b, errUnpack
	}
	b1 := b[2:]
	if len(b1) < n {
		return b, errUnpack
	}
	r.Val = r.Val[0:0]
	for i := 0; i < n; i += 2 {
		var reg uint16
		b1 = uU16s(b1, &reg)
		r.Val = append(r.Val, reg)
	}
	return b1, nil
}

// ReqResWrReg is the write-single-register request and response. See
// [1],§6.6,pg.19
type ReqResWrReg struct {
	mbReqRes
	Addr uint16
	Val  uint16
}

func (r *ReqResWrReg) FnCode() FnCode { return WrReg }

func (r *ReqResWrReg) Pack(b []byte) ([]byte, error) {
	b = append(b, byte(WrReg))
	b = pU16s(b, r.Addr, r.Val)
	return b, nil
}

func (r *ReqResWrReg) Unpack(b []byte) ([]byte, error) {
	if b[0] != byte(WrReg) {
		return b, errUnpack
	}
	b = uU16s(b[1:], &r.Addr, &r.Val)
	return b, nil
}

// ReqResWrCoil is the write-single-coil request and response. See
// [1],§6.5,pg.17
type ReqResWrCoil struct {
	mbReqRes
	Addr   uint16
	Status bool
}

func (r *ReqResWrCoil) FnCode() FnCode { return WrCoil }

func (r *ReqResWrCoil) Pack(b []byte) ([]byte, error) {
	var val uint16
	if r.Status {
		val = 0xff00
	}
	b = append(b, byte(WrCoil))
	b = pU16s(b, r.Addr, val)
	return b, nil
}

func (r *ReqResWrCoil) Unpack(b []byte) ([]byte, error) {
	if b[0] != byte(WrCoil) {
		return b, errUnpack
	}
	var val uint16
	b1 := uU16s(b[1:], &r.Addr, &val)
	switch val {
	case 0xff00:
		r.Status = true
	case 0x0000:
		r.Status = false
	default:
		return b, errUnpack
	}
	return b1, nil
}

// ReqWrCoils is the write-multiple-coils request. See
// [1],§6.11,pg.29
type ReqWrCoils struct {
	mbReq
	Addr    uint16
	Num     uint16
	BitStat []byte
}

func (r *ReqWrCoils) FnCode() FnCode { return WrCoils }

func (r *ReqWrCoils) Pack(b []byte) ([]byte, error) {
	//n := len(r.BitStat)
	return b, nil
}

func (r *ReqWrCoils) Unpack(b []byte) ([]byte, error) {
	return b, nil
}

// ResWrCoils is the write-multiple-coils response. See
// [1],§6.11,pg.29
type ResWrCoils struct {
	mbRes
	Addr uint16
	Num  uint16
}

func (r *ResWrCoils) FnCode() FnCode { return WrCoils }

func (r *ResWrCoils) Pack(b []byte) ([]byte, error) {
	return b, nil
}

func (r *ResWrCoils) Unpack(b []byte) ([]byte, error) {
	return b, nil
}

// ReqWrRegs is the write-multiple-registers request. See
// [1],§6.12,pg.30
type ReqWrRegs struct {
	mbReq
	Addr uint16
	Val  []byte
}

func (r *ReqWrRegs) FnCode() FnCode { return WrRegs }

func (r *ReqWrRegs) Pack(b []byte) ([]byte, error) {
	return b, nil
}

func (r *ReqWrRegs) Unpack(b []byte) ([]byte, error) {
	return b, nil
}

// ResWrRegs is the write-multiple-registers response. See
// [1],§6.12,pg.30
type ResWrRegs struct {
	mbRes
	Addr uint16
	Num  uint16
}

func (r *ResWrRegs) FnCode() FnCode { return WrRegs }

func (r *ResWrRegs) Pack(b []byte) ([]byte, error) {
	return b, nil
}

func (r *ResWrRegs) Unpack(b []byte) ([]byte, error) {
	return b, nil
}
