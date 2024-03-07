package wsstat

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
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
	DNSLookupDone        time.Duration // Time to resolve DNS (might be redundant with DNSLookup)
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
	defer ws.conn.Close()
	for {
        // Although the message content is not used directly here,
		// calling NextReader is necessary to trigger the pong handler.
		if _, _, err := ws.conn.NextReader(); err != nil {
			break
		}
	}
}

// Dial establishes a new WebSocket connection using the custom dialer defined in this package.
// Sets result times: WSHandshake, WSHandshakeDone
func (wsStat *WSStat) Dial(url string) error {
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
// Wraps the gorilla/websocket WriteMessage and ReadMessage methods.
// Sets result times: MessageRoundTrip, FirstMessageResponse
func (ws *WSStat) SendMessage(messageType int, data []byte) ([]byte, error) {
	start := time.Now()
	if err := ws.conn.WriteMessage(messageType, data); err != nil {
		return nil, err
	}
	// Assuming immediate response
	ws.conn.SetReadDeadline(time.Now().Add(time.Second * 5))
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
// Wraps the gorilla/websocket WriteMessage and ReadMessage methods.
// Sets result times: MessageRoundTrip, FirstMessageResponse
func (ws *WSStat) SendMessageBasic() error {
	_, err := ws.SendMessage(websocket.TextMessage, []byte("Hello, WebSocket!"))
	if err != nil {
		return err
	}
	return nil
}

// WriteMessage sends a message through the WebSocket connection and measures the round-trip time.
// Wraps the gorilla/websocket WriteJSON and ReadJSON methods.
// Sets result times: MessageRoundTrip, FirstMessageResponse
func (ws *WSStat) SendMessageJSON(v interface{}) (interface{}, error) {
	start := time.Now()
	if err := ws.conn.WriteJSON(&v); err != nil {
		return nil, err
	}
	// Assuming immediate response
	ws.conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	var resp interface{}
	err := ws.conn.ReadJSON(&resp)
	if err != nil {
		return nil, err
	}
	//log.Printf("Received message: %s", resp)
	ws.Result.MessageRoundTrip = time.Since(start)
	ws.Result.FirstMessageResponse = ws.Result.WSHandshakeDone + ws.Result.MessageRoundTrip
	return resp, nil
}

// SendPing sends a ping message through the WebSocket connection and measures the round-trip time until the pong response.
// Wraps the gorilla/websocket SetPongHandler and WriteMessage methods.
// Sets result times: MessageRoundTrip, FirstMessageResponse
func (ws *WSStat) SendPing() error {
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
		ws.Result.MessageRoundTrip = time.Since(start)
	case <-timeout:
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
	start := time.Now()
	err := ws.conn.Close()
	ws.Result.ConnectionClose = time.Since(start)
	ws.Result.TotalTime = ws.Result.FirstMessageResponse + ws.Result.ConnectionClose
	return err
}

func (r *Result) durations() map[string]time.Duration {
	return map[string]time.Duration{
		"DNSLookup":        r.DNSLookup,
		"TCPConnection":    r.TCPConnection,
		"TLSHandshake":     r.TLSHandshake,
		"WSHandshake":      r.WSHandshake,
		"MessageRoundTrip": r.MessageRoundTrip,
		"ConnectionClose":  r.ConnectionClose,

		"DNSLookupDone":		r.DNSLookupDone,
		"TCPConnected":			r.TCPConnected,
		"TLSHandshakeDone":		r.TLSHandshakeDone,
		"WSHandshakeDone":		r.WSHandshakeDone,
		"FirstMessageResponse":	r.FirstMessageResponse,
		"TotalTime":			r.TotalTime,
	}
}

// Format formats stats result.
func (r Result) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "DNS lookup:     %4d ms\n",
				int(r.DNSLookup/time.Millisecond))
			fmt.Fprintf(&buf, "TCP connection: %4d ms\n",
				int(r.TCPConnection/time.Millisecond))
			fmt.Fprintf(&buf, "TLS handshake:  %4d ms\n",
				int(r.TLSHandshake/time.Millisecond))
			fmt.Fprintf(&buf, "WS handshake:   %4d ms\n",
				int(r.WSHandshake/time.Millisecond))
			fmt.Fprintf(&buf, "Msg round trip: %4d ms\n",
				int(r.MessageRoundTrip/time.Millisecond))

			if r.ConnectionClose > 0 {
				fmt.Fprintf(&buf, "Close time:     %4d ms\n\n",
					int(r.ConnectionClose/time.Millisecond))
			} else {
				fmt.Fprintf(&buf, "Close time:     %4s ms\n\n", "-")
			}

			fmt.Fprintf(&buf, "Name lookup done:   %4d ms\n",
				int(r.DNSLookupDone/time.Millisecond))
			fmt.Fprintf(&buf, "TCP connected:      %4d ms\n",
				int(r.TCPConnected/time.Millisecond))
			fmt.Fprintf(&buf, "TLS handshake done: %4d ms\n",
				int(r.TLSHandshakeDone/time.Millisecond))
			fmt.Fprintf(&buf, "WS handshake done:  %4d ms\n",
				int(r.WSHandshakeDone/time.Millisecond))
			fmt.Fprintf(&buf, "First msg response: %4d ms\n",
				int(r.WSHandshakeDone/time.Millisecond))

			if r.TotalTime > 0 {
				fmt.Fprintf(&buf, "Total:              %4d ms\n",
					int(r.TotalTime/time.Millisecond))
			} else {
				fmt.Fprintf(&buf, "Total:          %4s ms\n", "-")
			}
			io.WriteString(s, buf.String())
			return
		}

		fallthrough
	case 's', 'q':
		d := r.durations()
		list := make([]string, 0, len(d))
		for k, v := range d {
			// Handle when End function is not called
			if (k == "ConnectionClose" || k == "TotalTime") && r.ConnectionClose == 0 {
				list = append(list, fmt.Sprintf("%s: - ms", k))
				continue
			}
			list = append(list, fmt.Sprintf("%s: %d ms", k, v/time.Millisecond))
		}
		io.WriteString(s, strings.Join(list, ", "))
	}
}

// NewDialer initializes and returns a websocket.Dialer with customized dial functions to measure the connection phases.
// Sets result times: DNSLookup, TCPConnection, TLSHandshake, DNSLookupDone, TCPConnected, TLSHandshakeDone
func NewDialer(result *Result) *websocket.Dialer {
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
    result := &Result{}
	dialer := NewDialer(result) // Use the custom dialer we defined

    ws := &WSStat{
        dialer: dialer,
		Result: result,
    }

    return ws
}
