// Copyright (c) 2015, Nick Patavalis (npat@efault.net).
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

/*

Package modbus implements the MobBus protocol. This package provides
low-level functions and types for implementing ModBus protocol servers
(slaves) and clients (masters). Modbus over asynchronous serial lines
is supported (RTU and ASCII frame encodings) as well as ModBus over
TCP. Additionally, implementations of simple modbus clients and
servers are included.

Typical usage example: Using package modbus you can issue a
read-discrete-inputs requests to a modbus-over-serial slave (with
node-id 0x01) like this:

  // Open and configure the serial port
  // (see package: github.com/npat-efault/serial)
  p, err := serial.Open("/dev/ttyS0")
  if err != nil {
      log.Fatalf("Port open: %s", err)
  }
  err = p.Conf(serial.Conf{Baudrate: 9600}, ConfBaudrate)
  if err != nil {
      log.Fatalf("Port config: %s", err)
  }

  // Create a modbus client (master) and issue request
  mbcli := &modbus.SerCli{Conn: p}
  resp, err := mbcli.Do(0x01, &ReqRdInputs{Addr:0x07d2, Num:42}, nil)
  if err != nil {
      if e, ok := err.(modbus.ErrIO); ok {
          log.Fatalf("Port I/O error: %s", err)
      }
      if e, ok := err.(ResExc); ok {
          log.Printf("Exception response: %s", err)
      }
      log.Printf("Request failed: %s, err)
  }
  log.Printf("Received response: %+v", resp)

Package "serial" allows Read and Write operations on async-serial
ports (/dev/tty's) that support timeouts and deadlines. If you do not
wish to (or cannot) use package serial (or something equivalent) you
can use (as a last resort) standard file-io by mocking-up dummy
methods to set deadlines/timeouts (that do nothing). Unfortunatelly by
doing so you cannot have timeouts and retransmissions.

The modbus package is structured like this (from lower to higher-level
provisions):

Modbus ADUs and PDUs are stored in byte-slices. Named types are
defined for byte-slices keeping serial ADUs (SerADU), Tcp ADUs
(TcpADU) and PDUs (PDU). These types provide methods for accesing
basic ADU/PDU fields (node-id, function-code, CRC, etc)

Types (structures) are defined corresponding to ModBus requests and
responses (functions), like write-coils (ReqWrCoils),
read-discrete-inputs (ReqRdInputs), etc. All such types implement the
ReqRes interface. Additionally, all request types implement the Req
interface, and all response types implement the Res interface. This
makes it possible, for example, to define a function that accepts only
requests as an argument. All request and response types provide Pack
and Unpack methods for packing (marshaling) them to byte-slices, and
for unpacking (unmarshaling) them from byte-slices. For example, to
pack a ReqRdInputs request in byte-slice b:

    b, err = (&ReqRdInputs{Addr: 0x07fd, Num: 42}).Pack(b)
    if err != nil {
        log.Fatal(err)
    }

And to unpack the response that you know is stored in b:

    var r ResRdInputs
    b, err = &r.Unpack(b)
    if err != nil {
        log.Fatal(err)
    }

Higher level functions are included for preparing full request and
response ADUs (including headers and checksums): SerPack and
TcpPack. Example:

    req = ReqRdInputs{Coils: True, Addr: 0x07d, Num: 42}
    sadu, err := SerPack(nil, 0x01, &req) // 0x01 is the node-id
    if err != nil {
        log.Fatal(err)
    }
    if ! sadu.CheckCRC() {
        log.Fatal("Bad CRC!")
    }

Serial and TCP I/O functions are provided for reading request and
response ADUs (modbus frames/packets) from io.Readers (from serial
ports, or TCP connections):

    sadu, err := SerReadResADU(r, nil)
    if err != nil {
        log.Fatalf("Cannot receive response: %s", err)
    if ! sadu.CheckCRC() {
        log.Fatal("Bad CRC!")
    }
    if sadu.IsExc() {
        log.Printf("Exception response from %d: [%s:%s]",
            sadu.Node(), sadu.FnCode(), sadu.ExCode())
    } else {
        log.Printf("Normal response from %d, type: %s",
            sadu.Node(), sadu.FnCode())
    }

Finally, convenient, full implementations are included for
modbus-over-serial and modbus-over-TCP clients (masters) and servers
(slaves). See the example at the beginning of this section.


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
