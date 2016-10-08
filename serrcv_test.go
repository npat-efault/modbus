// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "testing"

func TestResFrameSz(t *testing.T) {
	for _, tst := range packTestData {
		if tst.req {
			continue
		}
		a, err := SerPack(nil, 0x01, tst.r)
		if err != nil {
			t.Fatalf("Cannot pack %T: %s", tst.r, err)
		}
		if !a.CheckCRC() {
			t.Fatalf("Failed CRC %T!", tst.r)
		}
		for i := 0; i < len(a); i++ {
			var length int
			var ok bool
			//length, ok := serResFrameSz(a[:i])
			if ok {
				if length != len(a) {
					t.Fatalf("Bad len for %T: %d != %d",
						tst.r, length, len(tst.b))
				}
				t.Logf("%T: @%d --> %d\n", tst.r, i, length)
				break
			}
		}
	}
}
