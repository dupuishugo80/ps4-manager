package discovery

import "testing"

func TestConsoleAddr(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		port int
		want string
	}{
		{"ipv4 default port", "192.168.1.10", 12800, "192.168.1.10:12800"},
		{"ipv4 other port", "10.0.0.42", 9090, "10.0.0.42:9090"},
		{"ipv6", "fe80::1", 12800, "[fe80::1]:12800"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			console := Console{IP: tc.ip, Port: tc.port}
			if got := console.Addr(); got != tc.want {
				t.Fatalf("Addr() = %q, want %q", got, tc.want)
			}
		})
	}
}
