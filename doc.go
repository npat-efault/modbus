// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

/*

Package modbus implements the MobBus protocol. This package provides
low-level functions and types for implementing ModBus protocol servers
(slaves) and clients (masters). Modbus over asynchronous serial lines
is supported (RTU and ASCII frame encodings) as well as ModBus over
TCP.

Modbus Protocol Specs

The modbus package was implemented based on the specifications and
guidelines in the following documents.

1. ModBus Application Protocol v1.1b
   http://www.modbus.org/docs/Modbus_Application_Protocol_V1_1b.pdf

2. Modbus Over Serial Line v1.02
   http://modbus.org/docs/Modbus_over_serial_line_V1_02.pdf

3. Modbus Messaging on TCP/IP v1.0b
   http://www.modbus.org/docs/Modbus_Messaging_Implementation_Guide_V1_0b.pdf

*/
package modbus
