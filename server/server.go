/*

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

  $ curl :8080/integra -d PWR01
  ok
  $ curl :8080/integra -d MVLUP
  ok

Up to 10 messages can be sent at once by separating them with newlines
in the request body. (Note that the $'string' form causes shells like
bash to replace occurrences of \n with newlines.) Example:

  $ curl :8080/integra -d $'PWR01\nMVLUP\nSLI03'
  ok

Example command to query the Integra device state by issuing a GET
request to /integra (returns JSON):

  $ curl :8080/integra
  {"MVL":"42","PWR":"01"}

Note thath the device state reported by GET /integra is not
necessarily complete; it is made up of the messages received from the
Integra device since the server was started. If desired values are
missing from the reported device state, it can be useful to send a
series of QSTN messages to populate the state:

  $ curl :8080/integra
  {}
  $ curl :8080/integra -d $'PWRQSTN\nMVLQSTN\nSLIQSTN'
  ok
  $ curl :8080/integra
  {"MVL":"42","PWR":"01","SLI":"03"}

*/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jhesch/integra"
)

var (
	httpaddr    = flag.String("httpaddr", ":8080", "HTTP listen address")
	integraaddr = flag.String("integraaddr", ":60128", "Integra device address")
	verbose     = flag.Bool("verbose", false, "Verbose logging")
)

// websocketRead blocks waiting for messages to arrive from the
// websocket connection and forwards them to the Integra device.
func websocketRead(wsConn *websocket.Conn, integraClient *integra.Client) {
	for {
		_, m, err := wsConn.ReadMessage()
		if err != nil {
			// Log errors, except for logging websocket
			// going away errors (they happen every time a
			// browser tab is closed).
			if !websocket.IsCloseError(err, websocket.CloseGoingAway) {
				log.Println("ReadMessage failed:", err)
			}
			return
		}
		var message integra.Message
		err = json.Unmarshal(m, &message)
		if err != nil {
			log.Println("Unmarshall failed:", err)
		}

		err = integraClient.Send(&message)
		if err != nil {
			log.Println("Send failed:", err)
			continue
		}
	}
}

// websocketWrite blocks waiting for messages to arrive from the
// Integra device and forwards them to the websocket connection.
func websocketWrite(wsConn *websocket.Conn, integraClient *integra.Client) {
	for {
		message, err := integraClient.Receive()
		if err != nil {
			if *verbose {
				log.Println("Receive failed:", err)
				log.Println("Closing websocket")
			}
			_ = wsConn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		err = wsConn.WriteJSON(message)
		if err != nil {
			log.Println("WriteJSON failed:", err)
			log.Println("Closing websocket")
			_ = wsConn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
	}
}

func serveWs(client *integra.Client, w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade failed:", err)
		return
	}
	defer conn.Close()

	go websocketWrite(conn, client)
	websocketRead(conn, client)
}

func serveIntegraPost(client *integra.Client, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("ReadAll failed:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	messages := bytes.Split(b, []byte("\n"))
	if len(messages) > 10 {
		http.Error(w, "Max messages (10) exceeded", http.StatusBadRequest)
		return
	}
	for i, messageBytes := range messages {
		message, err := integra.NewMessage(messageBytes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if i > 0 {
			time.Sleep(50 * time.Millisecond)
		}
		err = client.Send(message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	fmt.Fprintln(w, "ok")
}

func serveIntegra(client *integra.Client, w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		state, err := json.Marshal(client.State())
		if err != nil {
			log.Println("Marshal failed:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(state)
		if err != nil {
			log.Println("Write failed:", err)
			return
		}
	} else if r.Method == "POST" {
		serveIntegraPost(client, w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func main() {
	flag.Parse()

	device, err := integra.Connect(*integraaddr)
	if err != nil {
		log.Fatalln("integra.Connect failed:", err)
	}

	http.Handle("/", http.FileServer(http.Dir("server/public")))
	http.HandleFunc("/integra", func(w http.ResponseWriter, r *http.Request) {
		client := device.NewClient()
		defer client.Close()
		serveIntegra(client, w, r)
	})
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		client := device.NewClient()
		defer client.Close()
		serveWs(client, w, r)
	})
	log.Fatal(http.ListenAndServe(*httpaddr, nil))
}
