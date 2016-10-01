// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

// XDU is a type with pre-allocated capacity than can store any modbus
// serial or TCP ADU. Before using it you *must* initialize it with
// one of the ResetSerADU or ResetTcpADU methods; then you can append
// the ADU data to the Data byte-slice. XDU can also be used to
// "convert" a Serial ADU to a TCP ADU (and vice-versa) without
// copying the PDU data.
type XDU struct {
	buf [MaxADU + SerCRCSz]byte
	// Append you ADU data here, after initializing
	// with Reset[Ser|Tcp]ADU
	Data []byte
}

/*
   |<-------------- Modbus TCP ADU ------------------->|
   |                                                   |
   |            +---Unit / Node Id                     |
   |            V                                      |
   +----------+---+------------------------------------+
   | MBAP Header  |            Modbus PDU              |
   +----------+---+------------------------------------+

              +---+------------------------------------+---+---+
              |   |            Modbus PDU              |  CRC  |
              +---+------------------------------------+---+---+
              |                                                |
              | <----------------Modbus Serial ADU ----------->|
*/

// ResetSerADU must be called to initialize the XDU before filling it
// with serial ADU data. After this, data can be appended to
// x.Data. Calling it on an XDU that already contains data, discards
// them.
func (x *XDU) ResetSerADU() {
	x.Data = x.buf[TcpHeadSz-SerHeadSz : TcpHeadSz-SerHeadSz]
}

// ResetTcpADU must be called to initialize the XDU before filling it
// with TCP ADU data. After this, data can be appended to
// x.Data. Calling it on an XDU that already contains data, discards
// them.
func (x *XDU) ResetTcpADU() {
	x.Data = x.buf[0:0]
}

// Ser2TcpADU converts a serial ADU to a TCP ADU. The Transcation-Id
// field of the MBAP header for the TCP ADU must be provided.
func (x *XDU) Ser2TcpADU(trans uint16) {
	l := uint16(len(x.Data))
	x.Data = x.buf[0 : l+TcpHeadSz-SerHeadSz-SerCRCSz]
	TcpADU(x.Data).SetTrans(trans)
	TcpADU(x.Data).SetProto(0)
	TcpADU(x.Data).SetLen(l - 2)
}

// Tcp2SerADU converts a TCP ADU to a serial ADU.
func (x *XDU) Tcp2SerADU() {
	l := len(x.Data)
	x.Data = x.buf[TcpHeadSz-SerHeadSz : l]
	x.Data = SerAddCRC(x.Data)
}
