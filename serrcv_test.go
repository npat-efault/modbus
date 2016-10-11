// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import (
	"bytes"
	"testing"
	"time"
)

func TestResFrameSz(t *testing.T) {
	for _, tst := range packTestData {
		a, err := SerPack(nil, 0x01, tst.r)
		if err != nil {
			t.Fatalf("Cannot pack %T: %s", tst.r, err)
		}
		if !a.CheckCRC() {
			t.Fatalf("Failed CRC %T!", tst.r)
		}
		var sz sizer
		for i := 0; i < len(a)+1; i++ {
			var nrem int
			var ok bool
			if tst.req {
				nrem, ok = sz.sizeReq(a[:i])
			} else {
				nrem, ok = sz.sizeRes(a[:i])
			}
			if !ok {
				t.Fatalf("Unsupported %T", tst.r)
			}
			if nrem > len(a)-i {
				t.Fatalf("Bad size for %T: %d > %d",
					tst.r, nrem, len(a)-i)
			}
			if nrem == 0 {
				if i != len(a) {
					t.Fatalf("Bad len for %T: %d != %d",
						tst.r, i, len(a))
				}
				if tst.req {
					t.Logf("REQ %T: @%d --> %d\n",
						tst.r, i, len(a))
				} else {
					t.Logf("RES %T: @%d --> %d\n",
						tst.r, i, len(a))
				}
				break
			}
		}
	}
}

type BytesDeadlineR struct {
	*bytes.Reader
}

func (r *BytesDeadlineR) SetReadDeadline(d time.Time) error {
	return nil
}

func NewBytesDeadlineR(b []byte) *BytesDeadlineR {
	r := &BytesDeadlineR{}
	r.Reader = bytes.NewReader(b)
	return r
}

func TestSerReceiverRTUSimple(t *testing.T) {
	var b []byte

	for _, tst := range packTestData {
		var err error
		b, err = SerPack(b, 0x01, tst.r)
		if err != nil {
			t.Fatalf("Cannot pack %T: %s", tst.r, err)
		}
	}

	br := NewBytesDeadlineR(b)
	rcv := NewSerReceiverRTU(br)

	for _, tst := range packTestData {
		if tst.req {
			var a SerADU
			var err error
			a, err = rcv.ReceiveReq(a, time.Time{})
			if err != nil {
				t.Fatalf("Rcv fail for %T: %s", tst.r, err)
			}
			a1, err := SerPack(nil, 0x1, tst.r)
			if err != nil {
				t.Fatalf("Cannot pack %T: %s", tst.r, err)
			}
			if !bytes.Equal(a, a1) {
				t.Fatalf("Not equal for %T:\n\t"+
					"Received: %v\n\t"+
					"Expected: %v\n\t", tst.r, a, a1)
			}
			t.Logf("REQ %s: %d bytes", a.FnCode(), len(a))
		} else {
			var a SerADU
			var err error
			a, err = rcv.ReceiveRes(a, time.Time{})
			if err != nil {
				t.Fatalf("Rcv fail for %T: %s", tst.r, err)
			}
			a1, err := SerPack(nil, 0x1, tst.r)
			if err != nil {
				t.Fatalf("Cannot pack %T: %s", tst.r, err)
			}
			if !bytes.Equal(a, a1) {
				t.Fatalf("Not equal for %T:\n\t"+
					"Received: %v\n\t"+
					"Expected: %v\n\t", tst.r, a, a1)
			}
			t.Logf("RES %s: %d bytes", a.FnCode(), len(a))
		}
	}
}
