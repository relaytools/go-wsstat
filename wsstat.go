package wsstat

import (
	"context"
	"crypto/tls"
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
	NameLookup           time.Duration // Time to resolve DNS (might be redundant with DNSLookup)
	TCPConnected         time.Duration // Time until the TCP connection is established
	TLSHandshakeDone     time.Duration // Time until the TLS handshake is completed
	WSHandshakeDone      time.Duration // Time until the WS handshake is completed
	FirstMessageResponse time.Duration // Time until the first message is received
	TotalTime            time.Duration // Total time from opening to closing the connection
}

// WsStat wraps the gorilla/websocket package and includes latency measurements in Result.
type WSStat struct {
	conn   *websocket.Conn
	dialer *websocket.Dialer
	Result *Result
}

// NewDialer initializes and returns a websocket.Dialer with customized dial functions to measure the connection phases.
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
			result.NameLookup = result.DNSLookup
			result.TCPConnected = result.NameLookup + result.TCPConnection

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
			result.NameLookup = result.DNSLookup
			result.TCPConnected = result.NameLookup + result.TCPConnection
			result.TLSHandshakeDone = result.TCPConnected + result.TLSHandshake

			return tlsConn, nil
		},
	}
}

// Dial establishes a new WebSocket connection using the custom dialer defined in this package.
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
func (ws *WSStat) WriteReadMessage(messageType int, data []byte) ([]byte, error) {
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

// ReadMessage reads a message from the WebSocket connection.
func (ws *WSStat) ReadMessage() (int, []byte, error) {
	return ws.conn.ReadMessage()
}

// Close closes the WebSocket connection and measures the time taken to close the connection.
func (ws *WSStat) CloseConn() error {
	log.Println("Closing WebSocket connection")
	start := time.Now()
	err := ws.conn.Close()
	ws.Result.ConnectionClose = time.Since(start)
	ws.Result.TotalTime = ws.Result.FirstMessageResponse + ws.Result.ConnectionClose
	return err
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
