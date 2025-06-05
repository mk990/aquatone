package agents

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/mk990/aquatone/core"
)

type URLPublisher struct {
	session *core.Session
}

func NewURLPublisher() *URLPublisher {
	return &URLPublisher{}
}

func (d *URLPublisher) ID() string {
	return "agent:url_publisher"
}

func (a *URLPublisher) Register(s *core.Session) error {
	if err := s.EventBus.SubscribeAsync(core.TCPPort, a.OnTCPPort, false); err != nil {
		return fmt.Errorf("failed to subscribe to %s event: %w", core.TCPPort, err)
	}
	a.session = s
	return nil
}

func (a *URLPublisher) OnTCPPort(port int, host string) {
	a.session.Out.Debug("[%s] Received new open port on %s: %d\n", a.ID(), host, port)
	var url string
	isTLS, err := a.isTLS(port, host)
	if err != nil {
		a.session.Out.Debug("[%s] Error checking TLS for %s:%d: %v\n", a.ID(), host, port, err)
		// Default to http if TLS check fails
		url = HostAndPortToURL(host, port, "http")
	} else {
		if isTLS {
			url = HostAndPortToURL(host, port, "https")
		} else {
			url = HostAndPortToURL(host, port, "http")
		}
	}
	// EventBus.Publish does not return an error
	a.session.EventBus.Publish(core.URL, url)
}

func (a *URLPublisher) isTLS(port int, host string) (bool, error) {
	if port == 80 {
		return false, nil
	}

	if port == 443 {
		return true, nil
	}

	dialer := &net.Dialer{Timeout: time.Duration(*a.session.Options.HTTPTimeout) * time.Millisecond}
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", host, port), conf)
	if err != nil {
		return false, err
	}
	if err := conn.Close(); err != nil {
		// Log the error but still return true as the connection was successful
		a.session.Out.Debug("[%s] Error closing TLS connection for %s:%d: %v\n", a.ID(), host, port, err)
	}
	return true, nil
}
