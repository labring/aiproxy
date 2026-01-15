package common

import (
	"context"
	"errors"
	"net"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

// connectionErrorPatterns contains string patterns that indicate connection errors
// Used as fallback for database drivers that may wrap errors in unexpected ways
var connectionErrorPatterns = []string{
	"dial error",
	"connection refused",
	"connection reset",
	"operation not permitted",
	"failed to connect",
	"dial tcp",
	"no such host",
	"i/o timeout",
	"network is unreachable",
	"host is unreachable",
	"connection timed out",
}

// IsDBConnectionError checks if the error is a database connection error
// using type-based error checking with errors.As, with string pattern fallback
func IsDBConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context errors (timeout, cancelled)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	// Check for pgx/pgconn connection errors
	var pgConnectErr *pgconn.ConnectError
	if errors.As(err, &pgConnectErr) {
		return true
	}

	// Check for pgx/pgconn timeout errors
	if pgconn.Timeout(err) {
		return true
	}

	// Check for redis connection errors
	if errors.Is(err, redis.ErrClosed) ||
		errors.Is(err, redis.ErrPoolExhausted) ||
		errors.Is(err, redis.ErrPoolTimeout) {
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

	// Fallback: string pattern matching for other drivers
	errStr := strings.ToLower(err.Error())
	for _, pattern := range connectionErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
