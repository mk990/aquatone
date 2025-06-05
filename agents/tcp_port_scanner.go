package agents

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mk990/aquatone/core"
)

// TCPPortScanner is responsible for scanning TCP ports on discovered hosts
type TCPPortScanner struct {
	session    *core.Session
	scanWorker chan struct{} // semaphore for limiting concurrent scans
}

// NewTCPPortScanner creates a new TCP port scanner agent
func NewTCPPortScanner() *TCPPortScanner {
	return &TCPPortScanner{}
}

// ID returns the identifier of the agent
func (d *TCPPortScanner) ID() string {
	return "agent:tcp_port_scanner"
}

// Register registers the agent with the session
func (a *TCPPortScanner) Register(s *core.Session) error {
	if err := s.EventBus.SubscribeAsync(core.Host, a.OnHost, false); err != nil {
		return fmt.Errorf("failed to subscribe to %s event: %w", core.Host, err)
	}
	a.session = s

	// Initialize worker pool with configurable size
	// Default to number of threads if available, otherwise use 100 as default
	concurrentScans := 100
	if a.session.Options.Threads != nil {
		concurrentScans = *a.session.Options.Threads
	}
	a.scanWorker = make(chan struct{}, concurrentScans)

	return nil
}

// OnHost is triggered when a new host is discovered
func (a *TCPPortScanner) OnHost(host string) {
	a.session.Out.Debug("[%s] Received new host: %s\n", a.ID(), host)

	// Resolve the host first to ensure it exists and to get IP addresses
	ips, err := net.LookupHost(host)
	if err != nil {
		a.session.Out.Error("[%s] Failed to resolve host %s: %v\n", a.ID(), host, err)
		return
	}

	a.session.Out.Debug("[%s] Successfully resolved %s to %v\n", a.ID(), host, ips)

	var wg sync.WaitGroup
	for _, port := range a.session.Ports {
		a.session.WaitGroup.Add()
		wg.Add(1)

		go func(port int, host string) {
			defer a.session.WaitGroup.Done()
			defer wg.Done()

			// Acquire worker slot
			a.scanWorker <- struct{}{}
			defer func() { <-a.scanWorker }()

			// Create context with timeout
			timeout := time.Duration(*a.session.Options.ScanTimeout) * time.Millisecond
			if timeout < 5*time.Second {
				// Ensure minimum timeout is reasonably long
				timeout = 5 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Try multiple times for reliability
			success := false
			for attempts := 0; attempts < 2 && !success; attempts++ {
				if attempts > 0 {
					a.session.Out.Debug("[%s] Retrying port %d on %s (attempt %d)\n", a.ID(), port, host, attempts+1)
					time.Sleep(500 * time.Millisecond) // Short delay between retries
				}

				if a.scanPort(ctx, port, host) {
					success = true
				}
			}

			if success {
				a.session.Stats.IncrementPortOpen()
				a.session.Out.Info("%s: port %s %s\n", host, Green(fmt.Sprintf("%d", port)), Green("open"))
				a.session.EventBus.Publish(core.TCPPort, port, host)
			} else {
				a.session.Stats.IncrementPortClosed()
				a.session.Out.Debug("[%s] Port %d is closed on %s\n", a.ID(), port, host)
			}
		}(port, host)
	}

	// Wait for all port scans to complete for this host
	go func() {
		wg.Wait()
		a.session.Out.Debug("[%s] Completed scanning all ports for host: %s\n", a.ID(), host)
	}()
}

// scanPort attempts to connect to a specific port on a host with context-based timeout
func (a *TCPPortScanner) scanPort(ctx context.Context, port int, host string) bool {
	// Increase the default timeout for the connection
	timeout := time.Duration(*a.session.Options.ScanTimeout) * time.Millisecond
	if timeout < 5*time.Second {
		// Ensure minimum timeout is reasonably long for internet connections
		timeout = 5 * time.Second
	}

	// Create a dialer with configurable options
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 0,    // No keep-alive for port scans
		DualStack: true, // Enable both IPv4 and IPv6
	}

	// Use DialContext for better timeout control
	target := fmt.Sprintf("%s:%d", host, port)
	a.session.Out.Debug("[%s] Attempting to connect to %s with timeout %v\n", a.ID(), target, timeout)

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		// Check if it's a timeout error
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			a.session.Out.Debug("[%s] Timeout scanning port %d on %s\n", a.ID(), port, host)
		} else {
			a.session.Out.Debug("[%s] Error scanning port %d on %s: %v\n", a.ID(), port, host, err)
		}
		return false
	}

	if conn != nil {
		defer func() {
			if err := conn.Close(); err != nil {
				a.session.Out.Debug("[%s] Error closing connection for %s:%d: %v\n", a.ID(), host, port, err)
			}
		}()
		// Try to read a byte to confirm the connection is truly established
		// Some firewalls might allow the initial handshake but drop subsequent packets
		one := make([]byte, 1)
		if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			a.session.Out.Debug("[%s] Error setting read deadline for %s:%d: %v\n", a.ID(), host, port, err)
			// Depending on policy, we might still consider the port open if SetReadDeadline fails
			// For now, let's assume it's a critical failure for this check.
			return false
		}
		_, err = conn.Read(one)
		// It's OK if we can't read (connection refused, EOF, timeout),
		// the fact that DialContext succeeded and SetReadDeadline was OK is enough.
		// We log the read error for debugging but still return true.
		if err != nil {
			a.session.Out.Debug("[%s] Error reading from connection for %s:%d (this is often expected): %v\n", a.ID(), host, port, err)
		}
		return true
	}

	return false
}
