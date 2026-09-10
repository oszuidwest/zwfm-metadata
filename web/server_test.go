package web

import (
	"context"
	"errors"
	"net"
	"testing"

	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

func TestServerStartReturnsBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", ":0") //nolint:gosec // Test-only listener reserves a wildcard port and serves no data.
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	server := &Server{
		port:         listener.Addr().(*net.TCPAddr).Port,
		router:       core.NewMetadataRouter(),
		dashboardHub: utils.NewWebSocketHub("test"),
	}
	err = server.Start(context.Background())
	if _, ok := errors.AsType[*net.OpError](err); !ok {
		t.Fatalf("Start() error = %v, want bind failure", err)
	}
}
