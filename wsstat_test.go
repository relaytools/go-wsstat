package wsstat

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var (	
	serverAddr = "localhost:8080"
	echoServerAddrWs = "ws://" + serverAddr + "/echo"
)

// TestMain sets up the test server and runs the tests in this file.
func TestMain(m *testing.M) {
	// Set up test server
	go func() {
		StartEchoServer(serverAddr)
	}()

	// Ensure the echo server starts before any tests run
	time.Sleep(250 * time.Millisecond)

	// Run the tests in this file
	os.Exit(m.Run())
}

func TestMeasureLatency(t *testing.T) {
	msg := "Hello, world!"
	result, response, err := MeasureLatency(echoServerAddrWs, msg)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.TotalTime <= 0 {
		t.Errorf("Invalid total time: %v", result.TotalTime)
	}
	if len(response) == 0 {
		t.Errorf("Empty response")
	}
	if string(response) != msg {
		t.Errorf("Unexpected response: %s", response)
	}
}

func TestMeasureLatencyJSON(t *testing.T) {
	message := struct {
		Text string `json:"text"`
	}{
		Text: "Hello, world!",
	}
	result, response, err := MeasureLatencyJSON(echoServerAddrWs, message)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.TotalTime <= 0 {
		t.Errorf("Invalid total time: %v", result.TotalTime)
	}
	if response == nil {
		t.Errorf("Empty response")
	}
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Errorf("Response is not a map")
		return
	}
	if responseMap["text"] != message.Text {
		t.Errorf("Unexpected response: %s", responseMap)
	}
}

func TestMeasureLatencyPing(t *testing.T) {
	result, err := MeasureLatencyPing(echoServerAddrWs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.TotalTime <= 0 {
		t.Errorf("Invalid total time: %v", result.TotalTime)
	}
	if result.MessageRoundTrip <= 0 {
		t.Errorf("Invalid message round trip time: %v", result.MessageRoundTrip)
	}
	if result.FirstMessageResponse <= 0 {
		t.Errorf("Invalid first message response time: %v", result.FirstMessageResponse)
	}
}

func TestNewWSStat(t *testing.T) {
	ws := NewWSStat()

	if ws.dialer == nil {
		t.Error("Expected non-nil dialer")
	}
	if ws.Result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestDial(t *testing.T) {
	ws := NewWSStat()
	err := ws.Dial(echoServerAddrWs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if ws.Result.WSHandshake <= 0 {
		t.Error("Invalid WSHandshake time")
	}
	if ws.Result.WSHandshakeDone <= 0 {
		t.Error("Invalid WSHandshakeDone time")
	}
}

func TestWriteReadClose(t *testing.T) {
	ws := NewWSStat()
	err := ws.Dial(echoServerAddrWs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	message := []byte("Hello, world!")
	startTime, err := ws.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	_, receivedMessage, err := ws.ReadMessage(startTime)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if string(receivedMessage) != string(message) {
		t.Errorf("Received message does not match sent message")
	}
	if ws.Result.MessageRoundTrip <= 0 {
		t.Error("Invalid MessageRoundTrip time")
	}
	if ws.Result.FirstMessageResponse <= 0 {
		t.Error("Invalid FirstMessageResponse time")
	}

	err = ws.CloseConn()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if ws.Result.ConnectionClose <= 0 {
		t.Error("Invalid ConnectionClose time")
	}
	if ws.Result.TotalTime <= 0 {
		t.Error("Invalid TotalTime")
	}
}

// Helpers

// StartEchoServer starts a WebSocket server that echoes back any received messages.
func StartEchoServer(addr string) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins
		},
	}

	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		defer conn.Close()
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				// Only print error if it's not a normal closure
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					log.Println("read:", err)
				}
				break
			}
			err = conn.WriteMessage(mt, message)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	})

	fmt.Printf("Echo server started on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
