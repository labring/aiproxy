package common

import (
	"context"
	"errors"
	"net"
	"syscall"
)

// IsDBConnectionError checks if the error is a database connection error
// using type-based error checking with errors.As
func IsDBConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context errors (timeout, cancelled)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	// Check for network operation errors
	var netOpErr *net.OpError
	if errors.As(err, &netOpErr) {
		return true
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check for syscall errors (connection refused, reset, etc.)
	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) {
		switch syscallErr {
		case syscall.ECONNREFUSED, // connection refused
			syscall.ECONNRESET,   // connection reset by peer
			syscall.ECONNABORTED, // connection aborted
			syscall.ETIMEDOUT,    // connection timed out
			syscall.ENETUNREACH,  // network is unreachable
			syscall.EHOSTUNREACH, // host is unreachable
			syscall.EPERM,        // operation not permitted
			syscall.ENOENT:       // no such file or directory (for unix sockets)
			return true
		}
	}

	// Check for net.AddrError
	var addrErr *net.AddrError
	if errors.As(err, &addrErr) {
		return true
	}

	return false
}
