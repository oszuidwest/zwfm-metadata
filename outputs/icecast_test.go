package outputs

import (
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
			addr := server.Listener.Addr().(*net.TCPAddr)

			output := NewIcecastOutput("test", config.IcecastOutputConfig{Server: addr.IP.String(), Port: addr.Port, Mountpoint: "/live"})
			if err := output.sendToIcecast("Artist - Title"); (err != nil) != tt.wantErr {
				t.Fatalf("sendToIcecast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
