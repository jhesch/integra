// Package integra provides a client to communicate with an Integra
// (or Onkyo) A/V receiver device using the Integra Serial Control
// Protocol over Ethernet (eISCP).
package integra

import (
	"errors"
	"log"
	"net"
)

// State represents the known state of the Integra device. Keys are
// ISCP message commands that map to ISCP parameter values. Each pair
// reflects the most recently received value for the key. Example
// key/value pair: "PWR": "01".
type State map[string]string

// A Client is an Integra device network client.
type Client struct {
	conn  net.Conn
	txbuf eISCPPacket
	rxbuf eISCPPacket
	State State
}

// Connect establishes a connection to the Integra device and returns
// a new Client. Only one client at a time may be used to communicate
// with the device.
func Connect(address string) (*Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	// Note: since there can only be a single TCP connection to
	// the Integra device at a time, it's acceptable to reuse
	// transmit and receive buffers instead of creating new ones
	// for each communication.
	txbuf := newEISCPPacket()
	rxbuf := make(eISCPPacket, packetSize)
	return &Client{conn, txbuf, rxbuf, State{}}, nil
}

// Send sends the given message to the Integra device.
func (c *Client) Send(m *Message) error {
	err := c.txbuf.init(m.String())
	if err != nil {
		return err
	}
	n, err := c.conn.Write(c.txbuf)
	if err != nil {
		return err
	}
	log.Printf("Sent message %v (%v bytes)\n", m, n)
	return nil
}

// Receive blocks until a new message is received from the Integra
// device and returns the message.
func (c *Client) Receive() (*Message, error) {
	n, err := c.conn.Read(c.rxbuf)
	if err != nil {
		return nil, err
	}
	if err := c.rxbuf.check(endOfPacketRx); err != nil {
		log.Printf("Received bad packet (%v):%v", err, c.rxbuf.debugString())
		return nil, errors.New("received eISCP packet failed integrity check")
	}
	message := c.rxbuf.message()
	log.Printf("Received %v (%v bytes)\n", message, n)
	c.State[message.Command] = message.Parameter
	return message, nil
}
