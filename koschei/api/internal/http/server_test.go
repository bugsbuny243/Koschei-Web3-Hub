package http

import "testing"

func TestNewServerRegistersUniqueRoutes(t *testing.T) {
	server := NewServer(nil, "database unavailable", "admin-password", "", "")
	if server == nil {
		t.Fatal("expected a server handler")
	}
}
