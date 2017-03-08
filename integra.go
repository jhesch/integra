/*

Package integra provides a client to communicate with an Integra (or
Onkyo) A/V receiver device using the Integra Serial Control Protocol
over Ethernet (eISCP).

Example usage:

  device, _ := integra.Connect(":60128")
  client := device.NewClient()
  message := integra.Message{"PWR", "01"}
  client.Send(&message)
  message, _ = client.Receive()
  fmt.Println("Got message from Integra A/V receiver:", message)
  client.Close()

See server/server.go for a working example.

*/
package integra

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

// state represents the known state of the Integra device.
type state struct {
	sync.RWMutex
	m map[string]string
}

// Device represents the Integra device, e.g. an A/V receiver.
type Device struct {
	state   state
	conn    net.Conn
	txbuf   eISCPPacket
	rxbuf   eISCPPacket
	clients map[*Client]bool
	add     chan *Client
	remove  chan *Client
	send    chan *sendRequest
	receive chan *Message
	exit    chan int
}

// Connect establishes a connection to the Integra device and returns
// a new Device. Only one network peer (i.e., Device) may be used to
// communicate with the Integra device at a time.
func Connect(address string) (*Device, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	// Note: since there can only be a single TCP connection to
	// the Integra device at a time, it's acceptable to reuse
	// transmit and receive buffers instead of creating new ones
	// for each message sent and received.
	device := &Device{
		state:   state{m: make(map[string]string)},
		conn:    conn,
		txbuf:   newEISCPPacket(),
		rxbuf:   make(eISCPPacket, packetSize),
		clients: make(map[*Client]bool),
		add:     make(chan *Client),
		remove:  make(chan *Client),
		send:    make(chan *sendRequest),
		receive: make(chan *Message),
		exit:    make(chan int)}

	go device.receiveLoop()
	go device.mainLoop()

	return device, nil
}

func (d *Device) removeClient(client *Client, explicit bool) {
	// Check the map first to make it safe to call this method for a
	// client that was previously removed via the other removal path
	// (explicit/non-explicit). This can happen, for example, if a
	// client isn't set up to receive. The extra tolerance here
	// keeps the Client interface simple.
	if !d.clients[client] {
		return
	}
	if !explicit {
		// We didn't get here via the client's Close method. The
		// likely case is that a message was received from the
		// device and the client isn't set up to receive. This
		// is a valid case since some clients only need to send
		// and check state, so remove the client and move on.
		log.Printf("Client %p unable to receive\n", client)
	}
	log.Printf("Removing client %p\n", client)
	delete(d.clients, client)
	// Close channel to unblock client's Receive call (and
	// allow the goroutine that called it to shut down).
	close(client.receive)
}

// mainLoop runs in its own goroutine and is in charge of adding and
// removing clients and routing messages between clients and the
// device.
func (d *Device) mainLoop() {
	for {
		select {
		case client := <-d.add:
			log.Printf("Adding client %p\n", client)
			d.clients[client] = true
		case client := <-d.remove:
			d.removeClient(client, true)
		case request := <-d.send:
			err := d.txbuf.init(request.message.String())
			if err != nil {
				log.Println("init failed:", err)
				request.client.err <- err
				continue
			}
			n, err := d.conn.Write(d.txbuf)
			if err != nil {
				log.Println("Write failed:", err)
				request.client.err <- err
				continue
			}
			log.Printf("Sent message %v (%v bytes)\n", request.message, n)
			request.client.err <- err
		case message := <-d.receive:
			for client := range d.clients {
				select {
				case client.receive <- message:
				default:
					d.removeClient(client, false)
				}
			}
			log.Printf("Broadcast %v to %v clients\n", message, len(d.clients))
		case code := <-d.exit:
			os.Exit(code)
		}
	}
}

// receiveLoop runs in its own goroutine and blocks while waiting for
// new messages to arrive from the device. Received messages are
// forwarded over the device's receive channel.
func (d *Device) receiveLoop() {
	for {
		n, err := d.conn.Read(d.rxbuf)
		if err != nil {
			if err == io.EOF {
				log.Println("EOF read from device; shutting down")
				d.exit <- 1
			}
			log.Println("Read failed:", err)
			continue
		}
		if err := d.rxbuf.check(endOfPacketRx); err != nil {
			log.Printf("Received bad packet (%v):%v", err, d.rxbuf.debugString())
			continue
		}
		message, err := d.rxbuf.message()
		if err != nil {
			log.Println("message failed:", err)
		}
		log.Printf("Received %v (%v bytes)\n", message, n)

		d.state.Lock()
		d.state.m[message.Command] = message.Parameter
		d.state.Unlock()

		d.receive <- message
	}
}

// sendRequest is sent over device's send channel with a message from
// a Client and allows an error to be returned to the sender over its
// err channel.
type sendRequest struct {
	message *Message
	client  *Client
}

// A Client is an Integra device network client.
type Client struct {
	device  *Device
	receive chan *Message
	err     chan error
}

// NewClient returns a new Integra device client, ready to send and
// receive messages.
func (d *Device) NewClient() *Client {
	c := &Client{d, make(chan *Message), make(chan error)}
	d.add <- c
	return c
}

// Send sends the given message to the Integra device.
func (c *Client) Send(m *Message) error {
	c.device.send <- &sendRequest{m, c}
	return <-c.err
}

// Receive blocks until a new message is received from the Integra
// device and returns the message.
func (c *Client) Receive() (*Message, error) {
	m, ok := <-c.receive
	if !ok {
		return nil, errors.New("channel closed")
	}
	return m, nil
}

// State returns a map representing the known state of the Integra
// device. Keys are ISCP message commands that map to ISCP parameter
// values. Each pair reflects the most recently received value for the
// key. Example key:value pair: PWR:01.
//
// To populate the state with a desired command:paremeter pair, send
// the corresponding QSTN message (e.g., PWRQSTN) prior to calling
// this method. Note that it may be necessary to sleep for ~50ms in
// between.
func (c *Client) State() map[string]string {
	state := make(map[string]string)
	c.device.state.RLock()
	for k, v := range c.device.state.m {
		state[k] = v
	}
	c.device.state.RUnlock()
	return state
}

// Close removes the client Device. Client can no longer send or
// receive messages.
func (c *Client) Close() {
	c.device.remove <- c
}
