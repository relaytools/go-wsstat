package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gorilla/websocket"
	"github.com/jakobilobi/go-wsstat"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Fatalf("Usage: go run main.go URL")
	}
    wsURL := args[1]

	// basicExample(wsURL)
	detailedExample(wsURL)
}

func basicExample(wsURL string) {
	// TODO: implement basic example
	return
}

func detailedExample(wsURL string) {
	var err error

	// 1. Create a new WSStat instance
    ws := wsstat.NewWSStat()

	// 2. Establish a WebSocket connection
	// This triggers the DNS lookup, TCP connection, TLS handshake, and WebSocket handshake timers
	if err := ws.Dial(wsURL); err != nil {
		log.Fatalf("Failed to establish WebSocket connection: %v", err)
	}

	// 3. Send a message and wait for the response
	// This triggers the first message round trip timer
    // 3.a. Write and read a message
	msg := "Hello, WebSocket!"
	startTime, err := ws.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
        log.Fatalf("Failed to write message: %v", err)
    }
	msgType, p, err := ws.ReadMessage(startTime)
	if err != nil {
		log.Fatalf("Failed to read message: %v", err)
	}
	fmt.Printf("Received message '%s' of type '%d'\n\n", p, msgType)
	// 3.b. Send a custom message and wait for the response in a single function
    /* msg := "Hello, WebSocket!"
	p, err := ws.SendMessage(websocket.TextMessage, []byte(msg))
    if err != nil {
        log.Fatalf("Failed to send message: %v", err)
    }
	fmt.Printf("Received message: %s\n\n", p) */
	// 3.c. Send a JSON message and wait for the response
	/* var msg = map[string]interface{}{"json": "message", "compatible": "with", "your": "target", "ws": "server"}
	p, err := ws.SendMessageJSON(msg)
    if err != nil {
        log.Fatalf("Failed to send message: %v", err)
    }
	log.Printf("Received message: %s", p) */
	// 3.d. If not interested in the response, you can use the basic message sending method
	/* if err := ws.SendMessageBasic(); err != nil {
		log.Fatalf("Failed to send message: %v", err)
	} */
	// 3.e. Alternatively just send a ping message
	/* if err := ws.SendPing(); err != nil {
		log.Fatalf("Failed to send ping: %v", err)
	} */

	// 4. Close the WebSocket connection
	// This triggers the total connection time timer, which includes the call to close the connection
	err = ws.CloseConn()
	if err != nil {
		log.Fatalf("Failed to close WebSocket connection: %v", err)
	}

    // 5. Print the results
	fmt.Printf("%+v\n", ws.Result)
}