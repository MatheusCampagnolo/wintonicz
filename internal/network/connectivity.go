package network

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

const MaxConnectivityTimeout = 30 * time.Second

// ConnectivityResult describes one explicit DNS and TCP reachability check.
type ConnectivityResult struct {
	Target            string   `json:"target"`
	TargetResolved    bool     `json:"targetResolved"`
	ResolvedAddresses []string `json:"resolvedAddresses"`
	TCPConnected      bool     `json:"tcpConnected"`
	DurationMillis    int64    `json:"durationMillis"`
	FailureReason     string   `json:"failureReason,omitempty"`
}

// CheckConnectivity resolves a host when needed and attempts a TCP connection
// to the supplied host:port within the provided timeout.
func CheckConnectivity(ctx context.Context, target string, timeout time.Duration) (result ConnectivityResult, err error) {
	started := time.Now()
	result = ConnectivityResult{Target: target, ResolvedAddresses: []string{}}
	defer func() { result.DurationMillis = time.Since(started).Milliseconds() }()

	if timeout <= 0 || timeout > MaxConnectivityTimeout {
		return result, fmt.Errorf("timeout must be greater than zero and no more than %s", MaxConnectivityTimeout)
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil || host == "" {
		return result, fmt.Errorf("target must be a host and port, such as example.com:443")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return result, fmt.Errorf("target port must be a number from 1 to 65535")
	}

	checkContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var addresses []net.IPAddr
	if ip := net.ParseIP(host); ip != nil {
		addresses = []net.IPAddr{{IP: ip}}
	} else {
		addresses, err = net.DefaultResolver.LookupIPAddr(checkContext, host)
		if err != nil {
			result.FailureReason = fmt.Sprintf("resolve %s: %v", host, err)
			return result, nil
		}
	}
	if len(addresses) == 0 {
		result.FailureReason = fmt.Sprintf("resolve %s: no addresses returned", host)
		return result, nil
	}
	result.TargetResolved = true
	for _, address := range addresses {
		result.ResolvedAddresses = append(result.ResolvedAddresses, address.String())
	}

	dialer := net.Dialer{}
	var lastErr error
	for _, address := range addresses {
		connection, dialErr := dialer.DialContext(checkContext, "tcp", net.JoinHostPort(address.String(), port))
		if dialErr == nil {
			_ = connection.Close()
			result.TCPConnected = true
			return result, nil
		}
		lastErr = dialErr
		if checkContext.Err() != nil {
			break
		}
	}
	result.FailureReason = fmt.Sprintf("connect to %s: %v", target, lastErr)
	return result, nil
}
