// Copyright 2017 Jacob Hesch
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Mock Integra device server that echoes received messages back to
// client for testing. A hard-coded message ("PWR01") is sent to
// client when this server receives a HUP signal.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func printBytes(b []byte) {
	for i, v := range b {
		fmt.Printf("%#02x", v)
		if i%4 == 3 {
			fmt.Printf("\n")
			if i == 15 || i == 31 {
				fmt.Printf("\n")
			}
		} else {
			fmt.Print(" ")
		}
	}
}

func handleClient(conn net.Conn) {
	defer func() {
		signal.Ignore(syscall.SIGHUP)
		log.Println("Closing connection")
		_ = conn.Close()
	}()
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP)
	go func() {
		for {
			s := <-c
			log.Printf("Received %v signal\n", s)
			buffer := []byte{
				0x49, 0x53, 0x43, 0x50,
				0x00, 0x00, 0x00, 0x10,
				0x00, 0x00, 0x00, 0x08,
				0x01, 0x00, 0x00, 0x00,

				0x21, 0x31, 0x50, 0x57,
				0x52, 0x30, 0x31, 0x1a,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00}

			n, err := conn.Write(buffer)
			if err != nil {
				log.Println("Error writing:", err)
				return
			}
			log.Printf("Wrote %d bytes:\n", n)
			printBytes(buffer)
		}
	}()

	for {
		buffer := make([]byte, 32)
		n, err := conn.Read(buffer)
		if err != nil {
			log.Println("Error reading:", err)
			return
		}
		log.Printf("Read %d bytes:\n", n)
		printBytes(buffer)

		time.Sleep(40 * time.Millisecond)

		// Replace client Tx end of packet marker with client
		// Rx marker.
		buffer[16+buffer[11]-1] = 0x1a

		n, err = conn.Write(buffer)
		if err != nil {
			log.Println("Error writing:", err)
			return
		}
		log.Printf("Wrote %d bytes:\n", n)
		printBytes(buffer)
	}
}

func main() {
	log.SetOutput(os.Stdout)
	l, err := net.Listen("tcp", ":60128")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleClient(conn)
	}
}
