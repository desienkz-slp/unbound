package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// detectManagementIP resolves the real primary outbound IP of the server.
func detectManagementIP() string {
	// 1. Explicit environment override
	if ip := os.Getenv("NETORA_MGMT_IP"); ip != "" {
		return strings.TrimSpace(ip)
	}

	// 2. Query kernel routing table via UDP connect (no traffic sent over network)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok && !localAddr.IP.IsLoopback() {
			return localAddr.IP.String()
		}
	}

	// 3. Fallback: inspect network interfaces for first non-loopback IPv4
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
					return ip.String()
				}
			}
		}
	}

	return "127.0.0.1"
}

// detectActiveIPv4Addresses returns all non-loopback IPv4 addresses assigned to UP network interfaces.
func detectActiveIPv4Addresses() []string {
	var ips []string
	seen := make(map[string]bool)

	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
				ipStr := ip.String()
				if !seen[ipStr] {
					seen[ipStr] = true
					ips = append(ips, ipStr)
				}
			}
		}
	}
	return ips
}

// detectSystemSpecs probes real CPU cores, total physical RAM, and installed Unbound version.
func detectSystemSpecs() (cores int, ramMB int, unboundVer string) {
	cores = runtime.NumCPU()
	if cores < 1 {
		cores = 1
	}

	ramMB = 1024
	// Read Linux /proc/meminfo
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.Atoi(fields[1]); err == nil && kb > 0 {
						ramMB = kb / 1024
					}
				}
				break
			}
		}
	}

	// Read installed Unbound version
	unboundVer = "1.13.1"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "unbound", "-V")
	if out, err := cmd.CombinedOutput(); err == nil {
		lines := strings.Split(string(out), "\n")
		if len(lines) > 0 {
			fields := strings.Fields(lines[0])
			for i, f := range fields {
				if strings.EqualFold(f, "Version") && i+1 < len(fields) {
					unboundVer = fields[i+1]
					break
				}
			}
		}
	}

	return cores, ramMB, unboundVer
}
