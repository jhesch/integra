# integra
Go [library](#library) and [web server](#server) to communicate with
Integra (or Onkyo) A/V receivers using the Integra Serial Control Protocol
over Ethernet (eISCP)

## Library

Package integra provides a client to communicate with an Integra (or
Onkyo) A/V receiver device using the Integra Serial Control Protocol
over Ethernet (eISCP).

Example usage:

```
  device, _ := integra.Connect(":60128")
  client := device.NewClient()
  message := integra.Message{"PWR", "01"}
  client.Send(&message)
  message, _ = client.Receive()
  fmt.Println("Got message from Integra A/V receiver:", message)
  client.Close()
```

See [server/server.go](server/server.go) for a working example.

## Server


Server provides a basic mobile-friendly web app to control and monitor
an Integra device such as an A/V receiver. The web app uses WebSockets
to display real-time changes to the device, including changes made
elsewhere like the volume knob on the receiver or buttons on the
remote.

Server also offers a simple HTTP interface at /integra for sending
ISCP (Integra Serial Control Protocol) messages and reading the
current state of the device.

The following examples assume this server is running on localhost port
8080.

Example commands to send ISCP power on (PWR01) and volume up (MVLUP)
messages to the device by issuing POST requests to /integra:

```
  $ curl :8080/integra -d PWR01
  ok
  $ curl :8080/integra -d MVLUP
  ok
```

Up to 10 messages can be sent at once by separating them with newlines
in the request body. (Note that the $'string' form causes shells like
bash to replace occurrences of \n with newlines.) Example:
```
  $ curl :8080/integra -d $'PWR01\nMVLUP\nSLI03'
  ok
```
Example command to query the Integra device state by issuing a GET
request to /integra (returns JSON):
```
  $ curl :8080/integra
  {"MVL":"42","PWR":"01"}
```
Note that the device state reported by GET /integra is not necessarily
complete; it is made up of the messages received from the Integra
device since the server was started. If desired values are missing
from the reported device state, it can be useful to send a series of
QSTN messages to populate the state:
```
  $ curl :8080/integra
  {}
  $ curl :8080/integra -d $'PWRQSTN\nMVLQSTN\nSLIQSTN'
  ok
  $ curl :8080/integra
  {"MVL":"42","PWR":"01","SLI":"03"}
```
