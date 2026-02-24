package notify

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// errBlockedAddress is returned when a webhook resolves to an address the
// operator must never call: loopback, link-local (cloud metadata) or
// unspecified. In-cluster and private addresses stay allowed.
var errBlockedAddress = errors.New("destination address is not allowed")

// NewHTTPClient returns the client notifiers post with: a timeout, no
// redirects, and a dialer that refuses blocked addresses after DNS
// resolution, so a hostname cannot point the operator at them either.
func NewHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, Control: refuseBlocked}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	transport.Proxy = nil
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func refuseBlocked(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("%w: %s", errBlockedAddress, host)
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() {
		return fmt.Errorf("%w: %s", errBlockedAddress, ip)
	}
	return nil
}
