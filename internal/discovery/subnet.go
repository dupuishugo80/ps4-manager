package discovery

import (
	"fmt"
	"net"
	"strconv"
)

const maxHostsPerSubnet = 254

type InterfaceLister interface {
	Interfaces() ([]net.Interface, error)
	Addrs(iface net.Interface) ([]net.Addr, error)
}

type systemInterfaceLister struct{}

func (systemInterfaceLister) Interfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

func (systemInterfaceLister) Addrs(iface net.Interface) ([]net.Addr, error) {
	return iface.Addrs()
}

// LocalHosts returns IPv4 addresses to probe, narrowing subnets wider than /24.
func LocalHosts(lister InterfaceLister) ([]string, error) {
	if lister == nil {
		lister = systemInterfaceLister{}
	}
	ifaces, err := lister.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}
	type subnet struct {
		ip   net.IP
		mask net.IPMask
	}
	var (
		subnets []subnet
		selfIPs = make(map[string]struct{})
	)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, addrsErr := lister.Addrs(iface)
		if addrsErr != nil {
			return nil, fmt.Errorf("list addrs for %s: %w", iface.Name, addrsErr)
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}
			subnets = append(subnets, subnet{ip: ip4, mask: ipNet.Mask})
			selfIPs[ip4.String()] = struct{}{}
		}
	}
	seen := make(map[string]struct{})
	var hosts []string
	for _, sub := range subnets {
		hosts = appendSubnetHosts(hosts, seen, selfIPs, sub.ip, sub.mask)
	}
	return hosts, nil
}

func appendSubnetHosts(dst []string, seen, selfIPs map[string]struct{}, ip net.IP, mask net.IPMask) []string {
	ones, bits := mask.Size()
	if bits != 32 || ones >= 31 {
		return dst
	}
	effective := mask
	if ones < 24 {
		effective = net.CIDRMask(24, 32)
	}
	network := ip.Mask(effective)
	broadcast := broadcastAddr(network, effective)
	count := 0
	for candidate := nextIP(network); !candidate.Equal(broadcast) && count < maxHostsPerSubnet; candidate = nextIP(candidate) {
		count++
		key := candidate.String()
		if _, isSelf := selfIPs[key]; isSelf {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		dst = append(dst, key)
	}
	return dst
}

func nextIP(ip net.IP) net.IP {
	out := make(net.IP, len(ip))
	copy(out, ip)
	for i := len(out) - 1; i >= 0; i-- {
		out[i]++
		if out[i] != 0 {
			break
		}
	}
	return out
}

func broadcastAddr(network net.IP, mask net.IPMask) net.IP {
	out := make(net.IP, len(network))
	for i := range network {
		out[i] = network[i] | ^mask[i]
	}
	return out
}

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
