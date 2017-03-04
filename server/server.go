package main

import (
	"encoding/json"
	"flag"
	"io"
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
	defer func() { _ = wsConn.Close() }()
	for {
		_, m, err := wsConn.ReadMessage()
		if err != nil {
			log.Println("ReadMessage (websocket) failed:", err)
			break
		}
		var message integra.Message
		err = json.Unmarshal(m, &message)
		if err != nil {
			log.Println("Unmarshall:", err)
		}

		log.Println("Websocket message:", message)
		err = integraClient.Send(&message)
		if err != nil {
			log.Println("Send (integra) failed:", err)
			continue
		}
	}
}

func websocketWrite(wsConn *websocket.Conn, integraClient *integra.Client) {
	defer func() { _ = wsConn.Close() }()
	for {
		message, err := integraClient.Receive()
		if err != nil {
			log.Println("Receive failed:", err)
			if err == io.EOF {
				log.Println("Closing websocket")
				_ = wsConn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			continue
		}
		log.Println(integraClient.State)
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
	log.Println("serveWs")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade failed:", err)
		return
	}

	go websocketWrite(conn, client)
	websocketRead(conn, client)
}

func serveIntegra(client *integra.Client, w http.ResponseWriter, r *http.Request) {
	state, err := json.Marshal(client.State)
	if err != nil {
		log.Println("Marshal failed:", err)
		http.Error(w, "Internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(state)
	if err != nil {
		log.Println("Write failed:", err)
	}
}

func main() {
	flag.Parse()

	client, err := integra.Connect(*integraaddr)
	if err != nil {
		log.Fatalln("integra.Connect failed:", err)
	}

	http.Handle("/", http.FileServer(http.Dir("server/public")))
	http.HandleFunc("/integra", func(w http.ResponseWriter, r *http.Request) {
		serveIntegra(client, w, r)
	})
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(client, w, r)
	})

	err = http.ListenAndServe(*httpaddr, nil)
	if err != nil {
		log.Fatalln("ListenAndServe failed:", err)
	}
}
