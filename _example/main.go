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
    ws := wsstat.NewWSStat()

	// Establish a WebSocket connection
	if err := ws.Dial(wsURL); err != nil {
		log.Fatalf("Failed to establish WebSocket connection: %v", err)
	}

    // Send a message and wait for a response
    msg := "Hello, WebSocket!"
	p, err := ws.WriteReadMessage(websocket.TextMessage, []byte(msg))
    if err != nil {
        log.Fatalf("Failed to send message: %v", err)
    }
	log.Printf("Received message: %s", p)

	err = ws.CloseConn()
	if err != nil {
		log.Fatalf("Failed to close WebSocket connection: %v", err)
	}

    // Print the results
    fmt.Println("DNS Lookup Time:", ws.Result.NameLookup)
    fmt.Println("TCP Connected:", ws.Result.TCPConnected)
    fmt.Println("TLS Handshake Done:", ws.Result.TLSHandshakeDone)
    fmt.Println("WS Handshake Done:", ws.Result.WSHandshakeDone)
    fmt.Println("First Message Round Trip Done:", ws.Result.FirstMessageResponse)
    fmt.Println("Total Connection Time:", ws.Result.TotalTime)
}
