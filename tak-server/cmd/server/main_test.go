package main

import (
	"net/http"
	"testing"
)

func TestHTTPServerTimeouts(t *testing.T) {
	api := apiHTTPServer(":0", http.NotFoundHandler())
	if api.ReadHeaderTimeout == 0 || api.ReadTimeout == 0 || api.WriteTimeout == 0 || api.IdleTimeout == 0 {
		t.Errorf("API server must time out slow clients: %+v", api)
	}
	if api.ReadTimeout < api.ReadHeaderTimeout {
		t.Errorf("ReadTimeout %v is shorter than ReadHeaderTimeout %v", api.ReadTimeout, api.ReadHeaderTimeout)
	}

	ws := websocketHTTPServer(":0", http.NotFoundHandler())
	if ws.ReadHeaderTimeout == 0 || ws.IdleTimeout == 0 {
		t.Errorf("WebSocket server must time out slow handshakes: %+v", ws)
	}
	// These cover the whole hijacked connection and would end every session.
	if ws.ReadTimeout != 0 || ws.WriteTimeout != 0 {
		t.Errorf("WebSocket server must not set ReadTimeout/WriteTimeout: %v, %v", ws.ReadTimeout, ws.WriteTimeout)
	}
}
