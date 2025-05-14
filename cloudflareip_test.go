package cloudflareip_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	plugin "github.com/imnoobincoding/cf-real"
)

// assertHeader prüft, ob der angegebene Header im Request den erwarteten Wert hat.
func assertHeader(t *testing.T, req *http.Request, key, expected string) {
	t.Helper() // Markiert diese Funktion als Test-Helfer
	if req.Header.Get(key) != expected {
		t.Errorf("Header '%s' - unerwarteter Wert: bekam '%s', erwartet '%s'", key, req.Header.Get(key), expected)
	}
}

func TestNew(t *testing.T) {
	cfg := plugin.CreateConfig()

	cfg.TrustIP = []string{"103.21.244.0/22"} // Beispielhafte TrustIPs für Tests

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	handler, err := plugin.New(ctx, next, cfg, "cloudflareip-test")
	if err != nil {
		t.Fatalf("Fehler beim Erstellen des Plugin-Handlers: %v", err)
	}

	testCases := []struct {
		desc                  string
		remoteAddr            string   // Die RemoteAddr, die der Traefik-Handler sieht (IP:Port)
		initialCfConnectingIP string   // Wert des Cf-Connecting-IP Headers bei Eingang
		initialXForwardedFor  string   // Optional: Initialer Wert von X-Forwarded-For
		initialXRealIP        string   // Optional: Initialer Wert von X-Real-IP
		expectedXForwardedFor string   // Erwarteter Wert von X-Forwarded-For nach dem Plugin
		expectedXRealIP       string   // Erwarteter Wert von X-Real-IP nach dem Plugin
	}{
		{
			desc:                  "Nicht vertrauenswürdige IP",
			remoteAddr:            "10.0.1.20:12345", // Nicht in cfg.TrustIP
			initialCfConnectingIP: "1.2.3.4",
			initialXForwardedFor:  "original-xff", // Sollte unverändert bleiben
			initialXRealIP:        "original-xri", // Sollte unverändert bleiben
			expectedXForwardedFor: "original-xff", // Bleibt unverändert
			expectedXRealIP:       "original-xri", // Bleibt unverändert
		},
		{
			desc:                  "Vertrauenswürdige IP, Cf-Connecting-IP vorhanden",
			remoteAddr:            "103.21.244.23:12345", // In cfg.TrustIP
			initialCfConnectingIP: "5.6.7.8",
			initialXForwardedFor:  "should-be-overwritten",
			initialXRealIP:        "should-also-be-overwritten",
			expectedXForwardedFor: "5.6.7.8", // Sollte mit Cf-Connecting-IP überschrieben werden
			expectedXRealIP:       "5.6.7.8", // Sollte mit Cf-Connecting-IP überschrieben werden
		},
		{
			desc:                  "Vertrauenswürdige IP, aber Cf-Connecting-IP ist leer",
			remoteAddr:            "103.21.244.23:54321", // In cfg.TrustIP
			initialCfConnectingIP: "",                    // Leer
			initialXForwardedFor:  "keep-this-xff",
			initialXRealIP:        "keep-this-xri",
			expectedXForwardedFor: "keep-this-xff", // Bleibt unverändert, da CfConnectingIP leer
			expectedXRealIP:       "keep-this-xri", // Bleibt unverändert, da CfConnectingIP leer
		},
		{
			desc:                  "Vertrauenswürdige IP (andere aus Liste), Cf-Connecting-IP vorhanden",
			remoteAddr:            "192.168.1.100:11223", // In cfg.TrustIP
			initialCfConnectingIP: "9.10.11.12",
			initialXForwardedFor:  "", // Initial leer
			initialXRealIP:        "", // Initial leer
			expectedXForwardedFor: "9.10.11.12",
			expectedXRealIP:       "9.10.11.12",
		},
		{
			desc:                  "RemoteAddr ohne Port, vertrauenswürdig",
			remoteAddr:            "103.21.244.1", // In cfg.TrustIP (SplitHostPort sollte damit umgehen)
			initialCfConnectingIP: "13.14.15.16",
			initialXForwardedFor:  "old-value",
			initialXRealIP:        "old-value-xri",
			expectedXForwardedFor: "13.14.15.16",
			expectedXRealIP:       "13.14.15.16",
		},
		{
			desc:                  "RemoteAddr ohne Port, nicht vertrauenswürdig",
			remoteAddr:            "10.0.0.1",      // Nicht in cfg.TrustIP
			initialCfConnectingIP: "17.18.19.20",
			initialXForwardedFor:  "preserved-xff",
			initialXRealIP:        "preserved-xri",
			expectedXForwardedFor: "preserved-xff",
			expectedXRealIP:       "preserved-xri",
		},
		{
			desc:                  "Ungültiges IP-Format für RemoteAddr",
			remoteAddr:            "this.is.not.an.ip:1234",
			initialCfConnectingIP: "21.22.23.24",
			initialXForwardedFor:  "original-A",
			initialXRealIP:        "original-B",
			expectedXForwardedFor: "original-A", // Keine Aktion vom Plugin erwartet
			expectedXRealIP:       "original-B", // Keine Aktion vom Plugin erwartet
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable für Goroutinen-Sicherheit in parallelen Tests (obwohl hier nicht parallel)
		t.Run(tc.desc, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
			if err != nil {
				t.Fatalf("Fehler beim Erstellen des Requests: %v", err)
			}

			// Setze RemoteAddr für den Test
			req.RemoteAddr = tc.remoteAddr

			// Setze initiale Header-Werte
			if tc.initialCfConnectingIP != "" {
				req.Header.Set("Cf-Connecting-IP", tc.initialCfConnectingIP)
			}
			if tc.initialXForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tc.initialXForwardedFor)
			}
			if tc.initialXRealIP != "" {
				req.Header.Set("X-Real-IP", tc.initialXRealIP)
			}

			// Führe den Handler aus
			handler.ServeHTTP(recorder, req)

			// Überprüfe die Header
			assertHeader(t, req, "X-Forwarded-For", tc.expectedXForwardedFor)
			assertHeader(t, req, "X-Real-IP", tc.expectedXRealIP)
		})
	}
}

// TestError prüft die Fehlerbehandlung bei ungültiger CIDR-Konfiguration.
// Diese Funktion kann aus deiner ursprünglichen Datei übernommen werden, da sie die New()-Funktion testet.
func TestError(t *testing.T) {
	cfg := plugin.CreateConfig()
	// Ungültiger CIDR-Block (z.B. /33)
	cfg.TrustIP = []string{"103.21.244.0/33"}

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	_, err := plugin.New(ctx, next, cfg, "cloudflareip-error-test")
	if err == nil {
		t.Fatalf("Erwarteter Fehler bei ungültigem CIDR, bekam aber keinen.")
	}
	// Optional: Genauer auf den Fehlertyp oder die Fehlermeldung prüfen,
	// z.B. if !strings.Contains(err.Error(), "invalid CIDR address") { ... }
}