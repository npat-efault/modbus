// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

type SerCliConf struct {
}

type SerCli struct {
}

func NewSerCli(port string, cfg SerCliConf) (*SerCli, error) {
	return &SerCli{}, nil
}

func (sc *SerCli) Close()

func (sc *SerCli) Reconf(cfg SerCliConf) error {
}

func (sc *SerCli) SndRcv(req SerADU, buf []byte) (SerAdu, error) {
}

func (sc *SerCli) Do(req Req, res Res) (Res, error) {
}
