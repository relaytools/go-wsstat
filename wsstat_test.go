package wsstat

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var (	
	serverAddr = "localhost:8080"
	echoServerAddrWs = "ws://" + serverAddr + "/echo"
	// TODO: support wss in tests
)

// TestMain sets up the test server and runs the tests in this file.
func TestMain(m *testing.M) {
	// Set up test server
	go func() {
		startEchoServer(serverAddr)
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
	validateDialResult(ws, echoServerAddrWs, getFunctionName(), t)
}

func TestWriteReadClose(t *testing.T) {
	ws := NewWSStat()
	err := ws.Dial(echoServerAddrWs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	validateDialResult(ws, echoServerAddrWs, getFunctionName(), t)

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
	validateSendResult(ws, getFunctionName(), t)

	err = ws.CloseConn()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	validateCloseResult(ws, getFunctionName(), t)
}

func TestSendMessage(t *testing.T) {
	ws := NewWSStat()
	err := ws.Dial(echoServerAddrWs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	validateDialResult(ws, echoServerAddrWs, getFunctionName(), t)

	message := []byte("Hello, world!")
	response, err := ws.SendMessage(websocket.TextMessage, message)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !bytes.Equal(response, message) {
		t.Errorf("Received message does not match sent message")
	}
	validateSendResult(ws, getFunctionName(), t)

	err = ws.CloseConn()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	validateCloseResult(ws, getFunctionName(), t)
}

func TestSendMessageJSON(t *testing.T) {
	ws := NewWSStat()
	err := ws.Dial(echoServerAddrWs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	validateDialResult(ws, echoServerAddrWs, getFunctionName(), t)

	message := struct {
		Text string `json:"text"`
	}{
		Text: "Hello, world!",
	}
	response, err := ws.SendMessageJSON(message)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Errorf("Response is not a map")
		return
	}
	if responseMap["text"] != message.Text {
		t.Errorf("Unexpected response: %s", responseMap)
	}
	validateSendResult(ws, getFunctionName(), t)

	err = ws.CloseConn()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	validateCloseResult(ws, getFunctionName(), t)
}


// Helpers

// getFunctionName returns the name of the calling function.
func getFunctionName() string {
    pc, _, _, _ := runtime.Caller(1)
    return strings.TrimPrefix(runtime.FuncForPC(pc).Name(), "main.")
}

// startEchoServer starts a WebSocket server that echoes back any received messages.
func startEchoServer(addr string) {
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

// Validation of WSStat results after Dial has been called
func validateDialResult(ws *WSStat, url string, msg string, t *testing.T) {
	if ws.Result.DNSLookup <= 0 {
		t.Errorf("Invalid WSHandshake time in %s", msg)
	}
	if ws.Result.DNSLookupDone <= 0 {
		t.Errorf("Invalid WSHandshakeDone time in %s", msg)
	}
	if ws.Result.TCPConnection <= 0 {
		t.Errorf("Invalid WSHandshake time in %s", msg)
	}
	if ws.Result.TCPConnected <= 0 {
		t.Errorf("Invalid WSHandshakeDone time in %s", msg)
	}
	if strings.Contains(url, "wss://") {
		if ws.Result.TLSHandshake <= 0 {
			t.Errorf("Invalid WSHandshake time in %s", msg)
		}
		if ws.Result.TLSHandshakeDone <= 0 {
			t.Errorf("Invalid WSHandshakeDone time in %s", msg)
		}
	}
	if ws.Result.WSHandshake <= 0 {
		t.Errorf("Invalid WSHandshake time in %s", msg)
	}
	if ws.Result.WSHandshakeDone <= 0 {
		t.Errorf("Invalid WSHandshakeDone time in %s", msg)
	}
}

// Validation of WSStat results after ReadMessage or SendMessage have been called
func validateSendResult(ws *WSStat, msg string, t *testing.T) {
	if ws.Result.MessageRoundTrip <= 0 {
		t.Errorf("Invalid MessageRoundTrip time in %s", msg)
	}
	if ws.Result.FirstMessageResponse <= 0 {
		t.Errorf("Invalid FirstMessageResponse time in %s", msg)
	}
}

// Validation of WSStat results after CloseConn has been called
func validateCloseResult(ws *WSStat, msg string, t *testing.T) {
	if ws.Result.ConnectionClose <= 0 {
		t.Errorf("Invalid ConnectionClose time in %s", msg)
	}
	if ws.Result.TotalTime <= 0 {
		t.Errorf("Invalid TotalTime in %s", msg)
	}
}
