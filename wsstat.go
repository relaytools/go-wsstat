package wsstat

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type Result struct {
	// Duration of each phase of the connection
	DNSLookup        time.Duration // Time to resolve DNS
	TCPConnection    time.Duration // TCP connection establishment time
	TLSHandshake     time.Duration // Time to perform TLS handshake
	WSHandshake      time.Duration // Time to perform WebSocket handshake
	MessageRoundTrip time.Duration // Time to send message and receive response
	ConnectionClose  time.Duration // Time to close the connection (complete the connection lifecycle)

	// Cumulative durations over the connection timeline
	DNSLookupDone           time.Duration // Time to resolve DNS (might be redundant with DNSLookup)
	TCPConnected         time.Duration // Time until the TCP connection is established
	TLSHandshakeDone     time.Duration // Time until the TLS handshake is completed
	WSHandshakeDone      time.Duration // Time until the WS handshake is completed
	FirstMessageResponse time.Duration // Time until the first message is received
	TotalTime            time.Duration // Total time from opening to closing the connection
}

// WSStat wraps the gorilla/websocket package and includes latency measurements in Result.
type WSStat struct {
	conn   *websocket.Conn
	dialer *websocket.Dialer
	Result *Result
}

// readLoop is a helper function to process received messages.
func (ws *WSStat) readLoop() {
	defer func() {
		ws.conn.Close()
	}()
	for {
		if _, _, err := ws.conn.NextReader(); err != nil {
			break
		}
		// Process received messages if necessary.
	}
}

// Dial establishes a new WebSocket connection using the custom dialer defined in this package.
// Sets result times: WSHandshake, WSHandshakeDone
func (wsStat *WSStat) Dial(url string) error {
	log.Println("Establishing WebSocket connection")
	start := time.Now()
	conn, _, err := wsStat.dialer.Dial(url, nil)
	if err != nil {
		return err
	}
	totalDialDuration := time.Since(start)
	wsStat.conn = conn
	wsStat.Result.WSHandshake = totalDialDuration - wsStat.Result.TLSHandshakeDone
	wsStat.Result.WSHandshakeDone = totalDialDuration
	return nil
}

// WriteMessage sends a message through the WebSocket connection and measures the round-trip time.
// Sets result times: MessageRoundTrip, FirstMessageResponse
func (ws *WSStat) SendMessage(messageType int, data []byte) ([]byte, error) {
	log.Println("Sending message through WebSocket connection")
	start := time.Now()
	if err := ws.conn.WriteMessage(messageType, data); err != nil {
		return nil, err
	}
	// Assuming immediate response; adjust logic as needed for your use case
	_, p, err := ws.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	//log.Printf("Received message: %s", p)
	ws.Result.MessageRoundTrip = time.Since(start)
	ws.Result.FirstMessageResponse = ws.Result.WSHandshakeDone + ws.Result.MessageRoundTrip
	return p, nil
}

// WriteReadMessageBasic sends a basic message through the WebSocket connection and measures the round-trip time.
// Sets result times: MessageRoundTrip, FirstMessageResponse
func (ws *WSStat) SendMessageBasic() error {
	log.Println("Sending basic message through WebSocket connection")
	_, err := ws.SendMessage(websocket.TextMessage, []byte("Hello, WebSocket!"))
	if err != nil {
		return err
	}
	return nil
}

// SendPing sends a ping message through the WebSocket connection and measures the round-trip time until the pong response.
// Sets result times: MessageRoundTrip, FirstMessageResponse
func (ws *WSStat) SendPing() error {
	log.Println("Sending ping message through WebSocket connection")

	pongReceived := make(chan bool) // Unbuffered channel
	timeout := time.After(5 * time.Second) // Timeout for the pong response

	ws.conn.SetPongHandler(func(appData string) error {
		pongReceived <- true
		return nil
	})

	go ws.readLoop() // Start the read loop to process the pong message

	start := time.Now()
	if err := ws.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		return err
	}

	select {
	case <-pongReceived:
		log.Println("Pong message received")
		ws.Result.MessageRoundTrip = time.Since(start)
	case <-timeout:
		log.Println("Pong response timeout")
		return errors.New("pong response timeout")
	}

	ws.Result.FirstMessageResponse = ws.Result.WSHandshakeDone + ws.Result.MessageRoundTrip
	return nil
}

// TODO: make implementation to use this or remove it
// ReadMessage reads a message from the WebSocket connection.
func (ws *WSStat) ReadMessage() (int, []byte, error) {
	return ws.conn.ReadMessage()
}

// Close closes the WebSocket connection and measures the time taken to close the connection.
// Sets result times: ConnectionClose, TotalTime
func (ws *WSStat) CloseConn() error {
	log.Println("Closing WebSocket connection")
	start := time.Now()
	err := ws.conn.Close()
	ws.Result.ConnectionClose = time.Since(start)
	ws.Result.TotalTime = ws.Result.FirstMessageResponse + ws.Result.ConnectionClose
	return err
}

// NewDialer initializes and returns a websocket.Dialer with customized dial functions to measure the connection phases.
// Sets result times: DNSLookup, TCPConnection, TLSHandshake, DNSLookupDone, TCPConnected, TLSHandshakeDone
func NewDialer(result *Result) *websocket.Dialer {
	log.Println("Creating new WebSocket dialer")
	return &websocket.Dialer{
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Perform DNS lookup
			dnsStart := time.Now()
			host, port, _ := net.SplitHostPort(addr)
			addrs, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, err
			}
			result.DNSLookup = time.Since(dnsStart)

			// Measure TCP connection time
			tcpStart := time.Now()
			// TODO: adjust timeout, possibly move to a configuration
			conn, err := net.DialTimeout(network, net.JoinHostPort(addrs[0], port), 3*time.Second)
			if err != nil {
				return nil, err
			}
			result.TCPConnection = time.Since(tcpStart)

			// Record the results
			result.DNSLookupDone = result.DNSLookup
			result.TCPConnected = result.DNSLookupDone + result.TCPConnection

			return conn, nil
		},

		NetDialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Perform DNS lookup
			dnsStart := time.Now()
			host, port, _ := net.SplitHostPort(addr)
			addrs, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, err
			}
			result.DNSLookup = time.Since(dnsStart)

			// Measure TCP connection time
			tcpStart := time.Now()
			dialer := &net.Dialer{}
			netConn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addrs[0], port))
			if err != nil {
				return nil, err
			}
			result.TCPConnection = time.Since(tcpStart)

			// Initiate TLS handshake over the established TCP connection
			tlsStart := time.Now()
			// TODO: Ensure proper TLS configuration; this implementation skips certificate verification
			tlsConn := tls.Client(netConn, &tls.Config{InsecureSkipVerify: true})
			err = tlsConn.Handshake()
			if err != nil {
				netConn.Close()
				return nil, err
			}
			result.TLSHandshake = time.Since(tlsStart)

			// Record the results
			result.DNSLookupDone = result.DNSLookup
			result.TCPConnected = result.DNSLookupDone + result.TCPConnection
			result.TLSHandshakeDone = result.TCPConnected + result.TLSHandshake

			return tlsConn, nil
		},
	}
}

// NewWSStat creates a new WSStat instance and establishes a WebSocket connection.
func NewWSStat() *WSStat {
	log.Println("Creating new WSStat instance")
    result := &Result{}
	dialer := NewDialer(result) // Use the custom dialer we defined

    ws := &WSStat{
        dialer: dialer,
		Result: result,
    }

    return ws
}
