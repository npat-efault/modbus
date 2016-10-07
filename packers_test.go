// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import (
	"bytes"
	"reflect"
	"testing"
)

type P struct {
	req bool
	b   []byte
	r   ReqRes
}

var packTestData = []P{
	// exception response
	P{
		false,
		[]byte{0x81, 0x01},
		&ResExc{
			Function: RdCoils,
			ExCode:   BadFnCode},
	},
	// read-coils request
	P{
		true,
		[]byte{0x01, 0x00, 0x13, 0x00, 0x13},
		&ReqRdInputs{
			Coils: true,
			Addr:  0x0013,
			Num:   0x0013},
	},
	// read-coils response
	P{
		false,
		[]byte{0x01, 0x03, 0xcd, 0x6b, 0x05},
		&ResRdInputs{
			Coils:   true,
			BitStat: []byte{0xcd, 0x6b, 0x05}},
	},
	// read-discrete-inputs request
	P{
		true,
		[]byte{0x02, 0x00, 0xc4, 0x00, 0x16},
		&ReqRdInputs{
			Coils: false,
			Addr:  0x00c4,
			Num:   0x0016},
	},
	// read-discrete-inputs response
	P{
		false,
		[]byte{0x02, 0x03, 0xac, 0xbd, 0x35},
		&ResRdInputs{
			Coils:   false,
			BitStat: []byte{0xac, 0xbd, 0x35}},
	},
	// read-holding-regs request
	P{
		true,
		[]byte{0x03, 0x00, 0x6b, 0x00, 0x03},
		&ReqRdRegs{
			Holding: true,
			Addr:    0x006b,
			Num:     0x0003},
	},
	// read-holding-regs response
	P{
		false,
		[]byte{0x03, 0x06, 0x02, 0x2b, 0x0, 0x0, 0x0, 0x64},
		&ResRdRegs{
			Holding: true,
			Val:     []uint16{0x022b, 0x0000, 0x0064}},
	},
	// read-input-regs request
	P{
		true,
		[]byte{0x04, 0x00, 0x6b, 0x00, 0x03},
		&ReqRdRegs{
			Holding: false,
			Addr:    0x006b,
			Num:     0x0003},
	},
	// read-input-regs response
	P{
		false,
		[]byte{0x04, 0x06, 0x02, 0x2b, 0x0, 0x0, 0x0, 0x64},
		&ResRdRegs{
			Holding: false,
			Val:     []uint16{0x022b, 0x0000, 0x0064}},
	},

	// write-single-coil request
	P{
		true,
		[]byte{0x05, 0x00, 0xac, 0xff, 0x00},
		&ReqResWrCoil{
			Addr:   0x00ac,
			Status: true},
	},
	//  write-single-coi response
	P{
		false,
		[]byte{0x05, 0x00, 0xac, 0xff, 0x00},
		&ReqResWrCoil{
			Addr:   0x00ac,
			Status: true},
	},
	// write-single-reg request
	P{
		true,
		[]byte{0x06, 0x00, 0xac, 0xde, 0xad},
		&ReqResWrReg{
			Addr: 0x00ac,
			Val:  0xdead},
	},
	//  write-single-reg response
	P{
		false,
		[]byte{0x06, 0x00, 0xac, 0xde, 0xad},
		&ReqResWrReg{
			Addr: 0x00ac,
			Val:  0xdead},
	},
}

func TestPackers(t *testing.T) {
	for _, tst := range packTestData {
		var b []byte
		b, err := tst.r.Pack(b)
		if err != nil {
			t.Fatalf("Cannot pack %T: %s", tst.r, err)
		}
		if len(b) != len(tst.b) {
			t.Fatalf("Bad pack %T. Bad length: %d != %d\n\t"+
				"pck: %x\n\t"+
				"exp: %x",
				tst.r, len(b), len(tst.b), b, tst.b)
		}
		if !bytes.Equal(b, tst.b) {
			t.Fatalf("Bad pack %T. No match\n\t"+
				"pck: %x\n\t"+
				"exp: %x",
				tst.r, b, tst.b)
		}
	}
}

func TestUnpackers(t *testing.T) {
	for _, tst := range packTestData {
		var r ReqRes
		var err error
		if tst.req {
			r, err = NewReq(FnCode(tst.b[0]))
		} else {
			r, err = NewRes(FnCode(tst.b[0]))
		}
		if err != nil {
			t.Fatalf("Bad fncode for %T: %#02x, %s", tst.r, tst.b[0], err)
		}
		b, err := r.Unpack(tst.b)
		if err != nil {
			t.Fatalf("Cannot unpack %T: %s", r, err)
		}
		if len(b) != 0 {
			t.Fatalf("Data leftover for %T:\n\t%x", r, b)
		}
		if !reflect.DeepEqual(r, tst.r) {
			t.Fatalf("Unpack doest not match %T:\n\t"+
				"upck: %v\n\t"+
				"exp:  %v\n\t", r, r, tst.r)
		}
	}
}

func TestSerPack(t *testing.T) {
	for _, tst := range packTestData {
		sadu, err := SerPack(nil, 0x01, tst.r)
		if err != nil {
			t.Fatalf("Cannot pack %T: %s", tst.r, err)
		}
		if sadu.Node() != 0x01 {
			t.Fatalf("%T: Bad node: %x\n", tst.r, sadu.Node())
		}
		if sadu.FnCode() != FnCode(tst.b[0] & ^ExcFlag) {
			t.Fatalf("%T: Bad FnCode: %s\n", tst.r, sadu.FnCode())
		}
		if !sadu.CheckCRC() {
			t.Fatalf("%T: Bad CRC: %x\n", tst.r, sadu.CRC())
		}
		if !bytes.Equal(sadu.PDU(), tst.b) {
			t.Fatalf("Pack does not match for %T:\n\t"+
				"pck: %v\n\t"+
				"exp:  %v\n\t", tst.r, sadu.PDU(), tst.b)
		}
	}
}
