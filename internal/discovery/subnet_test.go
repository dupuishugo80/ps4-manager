package discovery

import (
	"errors"
	"net"
	"slices"
	"testing"
)

type fakeInterfaceLister struct {
	ifaces   []net.Interface
	addrs    map[string][]net.Addr
	ifaceErr error
	addrsErr error
}

func (f fakeInterfaceLister) Interfaces() ([]net.Interface, error) {
	return f.ifaces, f.ifaceErr
}

func (f fakeInterfaceLister) Addrs(iface net.Interface) ([]net.Addr, error) {
	if f.addrsErr != nil {
		return nil, f.addrsErr
	}
	return f.addrs[iface.Name], nil
}

func mustParseCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse cidr %q: %v", cidr, err)
	}
	network.IP = ip
	return network
}

func TestLocalHostsSkipsLoopbackAndDown(t *testing.T) {
	lister := fakeInterfaceLister{
		ifaces: []net.Interface{
			{Name: "lo", Flags: net.FlagUp | net.FlagLoopback},
			{Name: "eth-down", Flags: 0},
			{Name: "eth0", Flags: net.FlagUp},
		},
		addrs: map[string][]net.Addr{
			"lo":       {mustParseCIDR(t, "127.0.0.1/8")},
			"eth-down": {mustParseCIDR(t, "10.0.0.1/24")},
			"eth0":     {mustParseCIDR(t, "192.168.1.42/24")},
		},
	}
	hosts, err := LocalHosts(lister)
	if err != nil {
		t.Fatalf("LocalHosts: %v", err)
	}
	if len(hosts) != 253 {
		t.Fatalf("expected 253 hosts (254 - self), got %d", len(hosts))
	}
	if slices.Contains(hosts, "192.168.1.42") {
		t.Fatalf("self IP should be excluded")
	}
	if !slices.Contains(hosts, "192.168.1.1") || !slices.Contains(hosts, "192.168.1.254") {
		t.Fatalf("expected range 1..254 in results")
	}
	if slices.Contains(hosts, "192.168.1.0") || slices.Contains(hosts, "192.168.1.255") {
		t.Fatalf("network and broadcast must be excluded")
	}
	if slices.Contains(hosts, "10.0.0.1") {
		t.Fatalf("down interface should be skipped")
	}
	if slices.Contains(hosts, "127.0.0.2") {
		t.Fatalf("loopback should be skipped")
	}
}

func TestLocalHostsNarrowsWideSubnet(t *testing.T) {
	lister := fakeInterfaceLister{
		ifaces: []net.Interface{{Name: "eth0", Flags: net.FlagUp}},
		addrs: map[string][]net.Addr{
			"eth0": {mustParseCIDR(t, "10.5.7.9/16")},
		},
	}
	hosts, err := LocalHosts(lister)
	if err != nil {
		t.Fatalf("LocalHosts: %v", err)
	}
	if len(hosts) != 253 {
		t.Fatalf("wide subnet should be capped at /24 (253 hosts), got %d", len(hosts))
	}
	if !slices.Contains(hosts, "10.5.7.1") || !slices.Contains(hosts, "10.5.7.254") {
		t.Fatalf("expected /24 around 10.5.7.9")
	}
	if slices.Contains(hosts, "10.5.8.1") {
		t.Fatalf("should not spill outside the /24 containing the host IP")
	}
}

func TestLocalHostsSkipsTinySubnets(t *testing.T) {
	lister := fakeInterfaceLister{
		ifaces: []net.Interface{{Name: "eth0", Flags: net.FlagUp}},
		addrs: map[string][]net.Addr{
			"eth0": {mustParseCIDR(t, "10.0.0.1/32")},
		},
	}
	hosts, err := LocalHosts(lister)
	if err != nil {
		t.Fatalf("LocalHosts: %v", err)
	}
	if len(hosts) != 0 {
		t.Fatalf("expected no hosts for /32, got %d", len(hosts))
	}
}

func TestLocalHostsDeduplicates(t *testing.T) {
	lister := fakeInterfaceLister{
		ifaces: []net.Interface{
			{Name: "eth0", Flags: net.FlagUp},
			{Name: "wlan0", Flags: net.FlagUp},
		},
		addrs: map[string][]net.Addr{
			"eth0":  {mustParseCIDR(t, "192.168.1.10/24")},
			"wlan0": {mustParseCIDR(t, "192.168.1.20/24")},
		},
	}
	hosts, err := LocalHosts(lister)
	if err != nil {
		t.Fatalf("LocalHosts: %v", err)
	}
	counts := make(map[string]int)
	for _, host := range hosts {
		counts[host]++
	}
	for host, count := range counts {
		if count > 1 {
			t.Fatalf("host %s emitted %d times, expected dedup", host, count)
		}
	}
	if slices.Contains(hosts, "192.168.1.10") || slices.Contains(hosts, "192.168.1.20") {
		t.Fatalf("self IPs of both interfaces should be excluded")
	}
}

func TestLocalHostsPropagatesError(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := LocalHosts(fakeInterfaceLister{ifaceErr: sentinel})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected error to wrap sentinel, got %v", err)
	}
}

func TestLocalHostsNilListerUsesSystem(t *testing.T) {
	if _, err := LocalHosts(nil); err != nil {
		t.Fatalf("nil lister should fallback to system, got %v", err)
	}
}

func TestNextIPAndBroadcast(t *testing.T) {
	ip := net.IPv4(192, 168, 1, 255).To4()
	next := nextIP(ip)
	if !next.Equal(net.IPv4(192, 168, 2, 0).To4()) {
		t.Fatalf("nextIP overflow failed: %v", next)
	}
	network := net.IPv4(10, 0, 0, 0).To4()
	mask := net.CIDRMask(24, 32)
	if !broadcastAddr(network, mask).Equal(net.IPv4(10, 0, 0, 255).To4()) {
		t.Fatalf("broadcastAddr wrong for /24")
	}
}
