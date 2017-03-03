package integra

// eISCP: Integra Serial Control Protocol over Ethernet
// https://www.google.com/search?q=eiscp
//
// eISCP protocol notes:
//
// - The data segment of a packet is fixed at 16 bytes (maxDataSize)
//   while the size of the data in the segment is variable and
//   determined by the byte at dataSizeIndex.
//
// - The first two bytes in the data segment of a packet are the start
//   character '!' and the unit type character ('1' for receiver).
//
// - The end of a packet received from the Integra device is marked
//   with 0x1a, while the end of a packet sent to the device is marked
//   with 0x0a.

import (
	"bytes"
	"errors"
	"fmt"
)

const (
	headerSize      byte = 16
	maxDataSize     byte = 16
	packetSize      byte = headerSize + maxDataSize
	headerSizeIndex byte = 7
	endOfPacketSize byte = 1
	dataStartSize   byte = 2 // data start: "!1"
	dataOverhead    byte = dataStartSize + endOfPacketSize
	dataSizeIndex   byte = 11
	maxMessageSize  byte = maxDataSize - dataOverhead
	messageOffset   byte = headerSize + dataStartSize
	endOfPacketTx   byte = 0x0a
	endOfPacketRx   byte = 0x1a
)

// eISCPPacket contains the bytes that make up a message sent to or
// received from an Integra device over Ethernet.
type eISCPPacket []byte

func newEISCPPacket() eISCPPacket {
	return eISCPPacket{
		0x49, 0x53, 0x43, 0x50, // I S C P
		0x00, 0x00, 0x00, 0x10, //      16 - header size
		0x00, 0x00, 0x00, 0x00, //       0 - data size
		0x01, 0x00, 0x00, 0x00, // 1       - ISCP version

		0x21, 0x31, 0x00, 0x00, // ! 1     - data start
		0x00, 0x00, 0x00, 0x00, // }
		0x00, 0x00, 0x00, 0x00, // } Empty message
		0x00, 0x00, 0x00, 0x00, // }
	}
}

// init initializes an outbound eISCPPacket that was created with
// newEISCPPacket with the given message.
func (p eISCPPacket) init(message string) error {
	if len(message) > int(maxMessageSize) {
		return fmt.Errorf("Message '%v' too long", message)
	}
	p[dataSizeIndex] = byte(len(message)) + dataOverhead
	index := messageOffset
	for i := range message {
		p[index] = message[i]
		index++
	}
	p[index] = endOfPacketTx
	for i := index + 1; i < packetSize; i++ {
		p[i] = 0x00
	}
	return nil
}

// message extracts the ISCP message from packet. The check method
// should be called to verify the packet's integrity before invoking
// message.
func (p eISCPPacket) message() *Message {
	messageSize := p[dataSizeIndex] - dataOverhead
	return newMessage(p[messageOffset : messageOffset+messageSize])
}

// check performs an integrity check on the packet.
func (p eISCPPacket) check(endOfPacket byte) error {
	switch {
	case p[0] != 0x49 || p[1] != 0x53 || p[2] != 0x43 || p[3] != 0x50:
		return errors.New("first 4 header bytes do not match ISCP")
	case p[headerSize] != 0x21 || p[headerSize+1] != 0x31:
		return errors.New("first 2 data bytes do not match !1")
	case p[headerSizeIndex] != headerSize:
		return fmt.Errorf("header size %#02x is not expected size %#02x",
			p[headerSizeIndex], headerSize)
	case p[dataSizeIndex] > maxDataSize:
		return fmt.Errorf("data size %#02x greater than max size %#02x",
			p[dataSizeIndex], maxDataSize)
	case p[headerSize+p[dataSizeIndex]-1] != endOfPacket:
		return fmt.Errorf("end of packet %#02x did not match expected value %#02x",
			p[headerSize+p[dataSizeIndex]-1], endOfPacket)
	}
	return nil
}

// debugString returns a multi-line, hex-formated string of packet's
// bytes. Note that it can be printed in a single line using the fmt
// package's %q format verb.
func (p eISCPPacket) debugString() string {
	buffer := bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("\n"))
	for i, b := range p {
		buffer.WriteString(fmt.Sprintf("%#02x", b))
		if i%4 == 3 {
			buffer.WriteString(fmt.Sprintf("\n"))
		} else {
			buffer.WriteString(fmt.Sprint(" "))
		}
	}
	return buffer.String()
}

// A Message is an ISCP message.
type Message struct {
	Command   string
	Parameter string
}

// String returns the message as a string.
func (m *Message) String() string {
	return m.Command + m.Parameter
}

// newMessage returns a new Message from the given byte slice making
// up the message's command and parameter.
func newMessage(m []byte) *Message {
	// Command is always the first three bytes of
	// message. Parameter is the remainer (variable length).
	return &Message{string(m[:3]), string(m[3:])}
}
