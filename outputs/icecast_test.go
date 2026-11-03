package outputs

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"zwfm-metadata/config"
)

func TestIcecastOutputChecksResponseMessage(t *testing.T) {
	tests := []struct {
		message string
		wantErr bool
	}{
		{message: "Metadata update successful"},
		{message: "Mountpoint will not accept URL updates", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("<?xml version=\"1.0\"?>\n<iceresponse><message>" + tt.message + "</message><return>1</return></iceresponse>\n"))
			}))
			t.Cleanup(server.Close)
			if err := newTestIcecastOutput(server, false).sendToIcecast("Artist - Title"); (err != nil) != tt.wantErr {
				t.Fatalf("sendToIcecast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIcecastOutputTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(server.Close)

	// The shared client does not trust the test certificate, so a certificate
	// error shows that the request was made over TLS.
	var certErr *tls.CertificateVerificationError
	if err := newTestIcecastOutput(server, true).sendToIcecast("Artist - Title"); !errors.As(err, &certErr) {
		t.Fatalf("sendToIcecast() error = %v, want a certificate verification error", err)
	}
}

func TestIcecastOutputDoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("redirect was followed")
	}))
	t.Cleanup(target.Close)
	server := httptest.NewServer(http.RedirectHandler(target.URL, http.StatusFound))
	t.Cleanup(server.Close)

	if err := newTestIcecastOutput(server, false).sendToIcecast("Artist - Title"); err == nil {
		t.Fatal("sendToIcecast() error = nil, want status error")
	}
}

func newTestIcecastOutput(server *httptest.Server, useTLS bool) *IcecastOutput {
	addr := server.Listener.Addr().(*net.TCPAddr)
	return NewIcecastOutput("test", config.IcecastOutputConfig{Server: addr.IP.String(), Port: addr.Port, Mountpoint: "/live", TLS: useTLS})
}
