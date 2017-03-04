package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/jhesch/integra"
)

var (
	httpaddr    = flag.String("httpaddr", ":8080", "HTTP listen address")
	integraaddr = flag.String("integraaddr", ":60128", "Integra device address")
)

func websocketRead(wsConn *websocket.Conn, integraClient *integra.Client) {
	for {
		_, m, err := wsConn.ReadMessage()
		if err != nil {
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

func websocketWrite(wsConn *websocket.Conn, integraClient *integra.Client) {
	for {
		message, err := integraClient.Receive()
		if err != nil {
			log.Println("Receive failed:", err)
			log.Println("Closing websocket")
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
		defer r.Body.Close()
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("ReadAll failed:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		message, err := integra.NewMessage(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = client.Send(message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "ok")
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

	err = http.ListenAndServe(*httpaddr, nil)
	if err != nil {
		log.Fatalln("ListenAndServe failed:", err)
	}
}
