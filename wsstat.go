package wsstat

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type Result struct {
	// Duration of each phase of the connection
	DNSLookup        time.Duration // Time to resolve DNS
	TLSHandshake     time.Duration // Time to perform TLS handshake
	WSHandshake      time.Duration // Time to perform WebSocket handshake
	TCPConnection    time.Duration // TCP connection establishment time
	MessageRoundTrip time.Duration // Time to send message and receive response
	ConnectionClose  time.Duration // Time to close the connection (complete the connection lifecycle)

	// Cumulative durations over the connection timeline
	NameLookup            time.Duration // Time to resolve DNS (might be redundant with DNSLookup)
	TCPConnected          time.Duration // Time until the TCP connection is established
	TLSHandshakeDone      time.Duration // Time until the TLS handshake is completed
	InitToMessageResponse time.Duration // Time until the first message is received
	TotalTime             time.Duration // Total time from opening to closing the connection
}

// WsStat wraps the gorilla/websocket package and includes latency measurements in Result.
type WSStat struct {
	conn   *websocket.Conn
	Result Result
}

// NewDialer initializes and returns a websocket.Dialer with customized dial functions to measure the connection phases.
func NewDialer() (*websocket.Dialer, *Result) {
	result := &Result{}
	return &websocket.Dialer{
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Start measuring from DNS lookup to the establishment of TCP connection.
			dnsStart := time.Now()

			// Perform DNS lookup
			host, port, _ := net.SplitHostPort(addr)
			addrs, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, err
			}
			dnsDuration := time.Since(dnsStart)
			result.DNSLookup = dnsDuration

			// Measure TCP connection time separately
			tcpStart := time.Now()
			conn, err := net.DialTimeout(network, net.JoinHostPort(addrs[0], port), 10*time.Second) // Adjust timeout as needed
			if err != nil {
				return nil, err
			}
			tcpDuration := time.Since(tcpStart)
			result.TCPConnection = dnsDuration + tcpDuration // This includes both DNS and TCP times

			return conn, nil
		},
		NetDialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Start measuring from DNS lookup to the establishment of TCP connection.
			dnsStart := time.Now()

			// Perform DNS lookup
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

			// Initiate TLS handshake over the established TCP connection.
			tlsStart := time.Now()
			tlsConn := tls.Client(netConn, &tls.Config{InsecureSkipVerify: true}) // Ensure proper TLS configuration
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
		// TODO: Any other Dialer fields to customize?
	}, result
}

// Dial establishes a new WebSocket connection using the custom dialer defined in this package.
func Dial(url string) (*WSStat, error) {
	dialer, _ := NewDialer()
	start := time.Now()
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	totalDialDuration := time.Since(start)
	ws := &WSStat{conn: conn}
	ws.Result.WSHandshake = totalDialDuration // This includes more than just the WS handshake; adjust accordingly
	return ws, nil
}

// WriteMessage sends a message through the WebSocket connection and measures the round-trip time.
func (ws *WSStat) WriteMessage(messageType int, data []byte) error {
	start := time.Now()
	if err := ws.conn.WriteMessage(messageType, data); err != nil {
		return err
	}
	// Assuming immediate response; adjust logic as needed for your use case
	_, _, err := ws.conn.ReadMessage()
	if err != nil {
		return err
	}
	ws.Result.MessageRoundTrip = time.Since(start)
	return nil
}

// ReadMessage reads a message from the WebSocket connection.
func (ws *WSStat) ReadMessage() (int, []byte, error) {
	return ws.conn.ReadMessage()
}

// Close closes the WebSocket connection and measures the time taken to close the connection.
func (ws *WSStat) Close() error {
	start := time.Now()
	err := ws.conn.Close()
	ws.Result.ConnectionClose = time.Since(start)
	return err
}
