package cloudflareip

import (
	"context"
	"log" // Für optionales Logging
	"net"
	"net/http"
)

const (
	xRealIP        = "X-Real-IP"        // Du kannst entscheiden, ob du diesen weiterhin setzen möchtest
	xForwardedFor  = "X-Forwarded-For"  // Der Header, den wir jetzt setzen wollen
	cfConnectingIP = "Cf-Connecting-IP" // Der Quell-Header von Cloudflare
)

// Config bleibt gleich
type Config struct {
	TrustIP []string `json:"trustip,omitempty" toml:"trustip,omitempty" yaml:"trustip,omitempty"`
}

// CreateConfig bleibt gleich
func CreateConfig() *Config {
	return &Config{
		TrustIP: []string{},
	}
}

// RealIPOverWriter bleibt strukturell gleich
type RealIPOverWriter struct {
	next    http.Handler
	name    string
	TrustIP []*net.IPNet
}

// New bleibt im Wesentlichen gleich, Fehlerbehandlung leicht verbessert
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.TrustIP) == 0 {
		log.Printf("[WARN] CloudflareIP Plugin (%s): TrustIP ist nicht konfiguriert. Das Plugin wird keinen Header überschreiben.", name)
	}

	ipOverWriter := &RealIPOverWriter{
		next:    next,
		name:    name,
		TrustIP: make([]*net.IPNet, 0, len(config.TrustIP)), // Kapazität initialisieren
	}

	for _, v := range config.TrustIP {
		_, trustip, err := net.ParseCIDR(v)
		if err != nil {
			log.Printf("[ERROR] CloudflareIP Plugin (%s): Ungültiger CIDR in TrustIP: %s - %v", name, v, err)
			return nil, err // Fehler weitergeben, um Traefik Start zu verhindern oder auf Problem hinzuweisen
		}
		ipOverWriter.TrustIP = append(ipOverWriter.TrustIP, trustip)
	}
	// log.Printf("[INFO] CloudflareIP Plugin (%s): Initialisiert mit TrustIPs: %v", name, config.TrustIP)
	return ipOverWriter, nil
}

func (r *RealIPOverWriter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	remoteAddrForTrustCheck := req.RemoteAddr // Die IP des direkten Peers (Sophos FW)

	// Loggen der eingehenden Header für Debugging-Zwecke (optional, im Produktivbetrieb ggf. entfernen)
	// log.Printf("[%s] Request from %s. Headers: Cf-Connecting-IP: '%s', X-Forwarded-For: '%s', X-Real-IP: '%s'",
	// 	r.name, remoteAddrForTrustCheck, req.Header.Get(cfConnectingIP), req.Header.Get(xForwardedFor), req.Header.Get(xRealIP))

	if r.isTrusted(remoteAddrForTrustCheck) {
		clientIPFromCF := req.Header.Get(cfConnectingIP)
		if clientIPFromCF != "" {
			// Setze X-Forwarded-For auf den Wert von Cf-Connecting-IP.
			// Dies überschreibt einen eventuell vorhandenen X-Forwarded-For Header.
			req.Header.Set(xForwardedFor, clientIPFromCF)
			// log.Printf("[%s] Trusted source %s. Set %s to %s", r.name, remoteAddrForTrustCheck, xForwardedFor, clientIPFromCF)

			// Optional: Setze auch X-Real-IP, falls deine Anwendung dies als Fallback nutzt
			// oder du beide Header konsistent halten möchtest.
			req.Header.Set(xRealIP, clientIPFromCF)
			// log.Printf("[%s] Trusted source %s. Set %s to %s", r.name, remoteAddrForTrustCheck, xRealIP, clientIPFromCF)

		} else {
			// log.Printf("[%s] Trusted source %s, but %s header is missing or empty.", r.name, remoteAddrForTrustCheck, cfConnectingIP)
		}
	} else {
		// log.Printf("[%s] Source %s is not trusted. Headers not modified by this plugin.", r.name, remoteAddrForTrustCheck)
	}

	r.next.ServeHTTP(rw, req)
}

// Umbenannt zu isTrusted und Fehlerbehandlung verbessert
func (r *RealIPOverWriter) isTrusted(remoteAddr string) bool {
	if remoteAddr == "" {
		// log.Printf("[%s] RemoteAddr is empty, cannot perform trust check.", r.name)
		return false
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// Wenn SplitHostPort fehlschlägt, könnte es bereits eine IP ohne Port sein.
		// log.Printf("[%s] Could not split host and port for %s (err: %v), assuming it's an IP.", r.name, remoteAddr, err)
		host = remoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// log.Printf("[%s] Could not parse IP from host %s (original RemoteAddr: %s).", r.name, host, remoteAddr)
		return false
	}

	for _, network := range r.TrustIP {
		if network.Contains(ip) {
			// log.Printf("[%s] IP %s is trusted by network %s.", r.name, ip, network)
			return true
		}
	}
	// log.Printf("[%s] IP %s is not in any trusted network.", r.name, ip)
	return false
}