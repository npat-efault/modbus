// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

// SerADU is a byte-slice holding a ModBus-over-TCP ADU
type TcpADU []byte

func (a TcpADU) Trans() uint16 { return uint16(a[0])<<8 | uint16(a[1]) }

func (a TcpADU) Proto() uint16 { return uint16(a[2])<<8 | uint16(a[3]) }

func (a TcpADU) Len() uint16 { return uint16(a[4])<<8 | uint16(a[5]) }

func (a TcpADU) Unit() uint8 { return a[6] }

func (a TcpADU) IsExc() bool { return a[TcpHeadSz+0]&ExcFlag != 0 }

func (a TcpADU) ExCode() ExCode { return ExCode(a[TcpHeadSz+1]) }

func (a TcpADU) FnCode() FnCode { return FnCode(a[TcpHeadSz+0] & ^ExcFlag) }

func (a TcpADU) PDU() PDU { return PDU(a[TcpHeadSz:]) }

func (a TcpADU) SetTrans(t uint16) {
	a[0] = byte(t >> 8)
	a[1] = byte(t)
}

func (a TcpADU) SetProto(p uint16) {
	a[2] = byte(p >> 8)
	a[3] = byte(p)
}

func (a TcpADU) SetLen(l uint16) {
	a[4] = byte(l >> 8)
	a[5] = byte(l)
}
