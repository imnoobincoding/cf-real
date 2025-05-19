package cloudflareip

import (
	"context"
	"fmt" // Hinzugefügt für fmt.Errorf
	"net"
	"net/http"
)

const (
	xRealIP        = "X-Real-IP"
	xForwardedFor  = "X-Forwarded-For"
	cfConnectingIP = "Cf-Connecting-IP"
)

type Config struct {
	TrustIP []string `json:"trustip,omitempty" toml:"trustip,omitempty" yaml:"trustip,omitempty"`
}

func CreateConfig() *Config {
	return &Config{
		TrustIP: []string{},
	}
}

type RealIPOverWriter struct {
	next    http.Handler
	name    string
	TrustIP []*net.IPNet
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	// Die Warnung kann für den Test entfernt oder durch fmt.Printf ersetzt werden, wenn du sie sehen willst.
	// if len(config.TrustIP) == 0 {
	// 	fmt.Printf("[WARN] CloudflareIP Plugin (%s): TrustIP ist nicht konfiguriert. Das Plugin wird keinen Header überschreiben.\n", name)
	// }

	ipOverWriter := &RealIPOverWriter{
		next:    next,
		name:    name,
		TrustIP: make([]*net.IPNet, 0, len(config.TrustIP)),
	}

	for _, v := range config.TrustIP {
		_, trustip, err := net.ParseCIDR(v)
		if err != nil {
			// Fehlerbehandlung ähnlich zum Original oder einfach 'return nil, err'
			return nil, fmt.Errorf("CloudflareIP Plugin (%s): Ungültiger CIDR in TrustIP '%s': %w", name, v, err)
		}
		ipOverWriter.TrustIP = append(ipOverWriter.TrustIP, trustip)
	}
	return ipOverWriter, nil
}

func (r *RealIPOverWriter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	remoteAddrForTrustCheck := req.RemoteAddr

	if r.isTrusted(remoteAddrForTrustCheck) {
		clientIPFromCF := req.Header.Get(cfConnectingIP)
		if clientIPFromCF != "" {
			req.Header.Set(xForwardedFor, clientIPFromCF)
			req.Header.Set(xRealIP, clientIPFromCF)
		}
	}
	r.next.ServeHTTP(rw, req)
}

func (r *RealIPOverWriter) isTrusted(remoteAddr string) bool {
	if remoteAddr == "" {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range r.TrustIP {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}