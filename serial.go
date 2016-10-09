// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "github.com/npat-efault/gohacks/crc16"

// SerADU is a byte-slice holding a ModBus serial ADU
type SerADU []byte

func (a SerADU) Node() uint8 { return a[0] }

func (a SerADU) IsExc() bool { return a[SerHeadSz+0]&ExcFlag != 0 }

func (a SerADU) ExCode() ExCode { return ExCode(a[SerHeadSz+1]) }

func (a SerADU) FnCode() FnCode { return FnCode(a[SerHeadSz+0] & ^ExcFlag) }

func (a SerADU) PDU() PDU { return PDU(a[SerHeadSz : len(a)-2]) }

// CRC returns the CRC stored in the serial ADU
func (a SerADU) CRC() uint16 {
	l := len(a)
	return uint16(a[l-2]) | uint16(a[l-1])<<8
}

// CheckCRC checks if the CRC stored in the serial ADU corresponds to
// the value calculated over the ADU's data.
func (a SerADU) CheckCRC() bool {
	crc := crc16.Checksum(crc16.Modbus, a[:len(a)-2])
	return a.CRC() == crc
}

// SerAddCRC appends a ModBus serial CRC16, calculated over the
// contents of byte-slice b, at the end of b. The CRC is appended
// less-important byte first. The resulting slice is returned as a
// modbus serial ADU.
func SerAddCRC(b []byte) SerADU {
	crc := crc16.Checksum(crc16.Modbus, b)
	b = append(b, byte(crc), byte(crc>>8))
	return b
}

// SerPack packs (marshals) a modbus serial request or response ADU
// and appends it to slice "b". It is ok if "b" is nil. Returns the
// appended-to slice as SerADU, or error. On error "b" is returned
// unaffected.
func SerPack(b []byte, node uint8, rr ReqRes) (SerADU, error) {
	b1 := append(b, node)
	b1, err := rr.Pack(b1)
	if err != nil {
		return b, err
	}
	crc := crc16.Checksum(crc16.Modbus, b1[len(b):])
	b1 = append(b1, byte(crc), byte(crc>>8))
	return b1, nil
}
