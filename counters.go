// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package modbus

import "sync"

type Counter int

const (
	SlvCntBusMsg Counter = iota
	SlvCntErrCRC
	SlvCntException
	SlvCntSlvMsg
	SlvCntSlvNoRes
	SlvCntSlvNAK
	SlvCntSlvBusy
	SlvCntOverrun

	SlvCntNum = iota
)

type counters struct {
	sync.Mutex
	ca []uint64
}

func (c *counters) Init(n int) {
	c.Lock()
	defer c.Unlock()
	c.ca = make([]uint64, n)
}

func (c *counters) Inc(cnt Counter) {
	c.Lock()
	defer c.Unlock()
	if int(cnt) >= len(c.ca) {
		return
	}
	c.ca[cnt]++
}

func (c *counters) Rst(cnt Counter) {
	c.Lock()
	defer c.Unlock()
	if int(cnt) >= len(c.ca) {
		return
	}
	c.ca[cnt] = 0
}

func (c *counters) Get(cnt Counter) uint64 {
	c.Lock()
	defer c.Unlock()
	if int(cnt) >= len(c.ca) {
		return 0
	}
	return c.ca[cnt]
}

func (c *counters) GetAll() []uint64 {
	c.Lock()
	defer c.Unlock()
	r := make([]uint64, len(c.ca))
	copy(r, c.ca)
	return r
}

func (c *counters) RstAll() {
	c.Lock()
	defer c.Unlock()
	for i, _ := range c.ca {
		c.ca[i] = 0
	}
}
