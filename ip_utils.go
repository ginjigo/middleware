package middleware

import (
	"net"
	"strings"
)

// isIPInCIDR checks if an IP address is within a CIDR range.
func isIPInCIDR(ipStr, cidr string) bool {
	// Parse CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		// Not a CIDR, do exact match
		return ipStr == cidr
	}

	// Parse IP (remove port if present)
	ip := ipStr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	return ipNet.Contains(parsedIP)
}
