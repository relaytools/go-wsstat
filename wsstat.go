package wsstat

import "time"

type Result struct {
	// Duration of each phase of the connection
	DNSLookup               time.Duration // Time to resolve DNS
	// TODO: consider adding TLS handshake time
	ConnectionEstablishment time.Duration // TCP connection + WS handshake
	MessageRoundTrip        time.Duration // Time to send message and receive response
	ConnectionClose         time.Duration // Time to close the connection (complete the connection lifecycle)

	// Cumulative durations over the connection timeline
	NameLookup            time.Duration // Time to resolve DNS (might be redundant with DNSLookup)
	Connected             time.Duration // Time until the connection is established
	InitToMessageResponse time.Duration // Time until the first message is received
	TotalTime             time.Duration // Total time from opening to closing the connection
}
