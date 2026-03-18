package utils

import (
	"net"
	"net/http"
	"strings"
)

// add trusted proxies here
var trustedProxies = []*net.IPNet{}

func parseCIDR(s string) *net.IPNet {
	_, ipnet, _ := net.ParseCIDR(s)
	return ipnet
}
func isTrustedProxy(ip net.IP) bool {
	for _, cidr := range trustedProxies {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
func GetIP(r *http.Request) string {
	// Check X-Forwarded-For first
	var candidateIP string
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		for i := len(ips) - 1; i >= 0; i-- {
			ipStr := strings.TrimSpace(ips[i])
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			// Stop at first untrusted proxy
			if !isTrustedProxy(ip) {
				candidateIP = ipStr
				break
			}
		}
	}
	// Fallback to X-Real-IP
	if candidateIP == "" {
		candidateIP = r.Header.Get("X-Real-IP")
		if ip := net.ParseIP(candidateIP); ip != nil {
			candidateIP = ip.String()
		}
	}
	// Final fallback to RemoteAddr
	if candidateIP == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			candidateIP = host
		} else {
			candidateIP = r.RemoteAddr
		}
	}
	// Normalize IP formats
	if ip := net.ParseIP(candidateIP); ip != nil {
		if ip.To4() != nil {
			return ip.String() // IPv4
		}
		return "[" + ip.String() + "]" // IPv6
	}
	return candidateIP
}
