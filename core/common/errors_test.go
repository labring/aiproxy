package common_test

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
)

func TestIsDBConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some random error"),
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: true,
		},
		{
			name:     "wrapped context deadline exceeded",
			err:      errors.Join(errors.New("operation failed"), context.DeadlineExceeded),
			expected: true,
		},
		{
			name:     "syscall connection refused",
			err:      syscall.ECONNREFUSED,
			expected: true,
		},
		{
			name:     "syscall connection reset",
			err:      syscall.ECONNRESET,
			expected: true,
		},
		{
			name:     "syscall connection timed out",
			err:      syscall.ETIMEDOUT,
			expected: true,
		},
		{
			name:     "syscall network unreachable",
			err:      syscall.ENETUNREACH,
			expected: true,
		},
		{
			name:     "syscall host unreachable",
			err:      syscall.EHOSTUNREACH,
			expected: true,
		},
		{
			name:     "syscall operation not permitted",
			err:      syscall.EPERM,
			expected: true,
		},
		{
			name: "net.OpError with connection refused",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: syscall.ECONNREFUSED,
			},
			expected: true,
		},
		{
			name: "net.DNSError",
			err: &net.DNSError{
				Err:  "no such host",
				Name: "example.com",
			},
			expected: true,
		},
		{
			name: "net.AddrError",
			err: &net.AddrError{
				Err:  "invalid address",
				Addr: "invalid",
			},
			expected: true,
		},
		{
			name: "wrapped net.OpError",
			err: errors.Join(
				errors.New("database query failed"),
				&net.OpError{
					Op:  "dial",
					Net: "tcp",
					Err: syscall.ECONNREFUSED,
				},
			),
			expected: true,
		},
		// pgconn type-based tests
		{
			name: "pgconn.ConnectError",
			err: &pgconn.ConnectError{
				Config: &pgconn.Config{Host: "localhost", Port: 5432},
			},
			expected: true,
		},
		{
			name: "wrapped pgconn.ConnectError",
			err: errors.Join(
				errors.New("database operation failed"),
				&pgconn.ConnectError{
					Config: &pgconn.Config{Host: "localhost", Port: 5432},
				},
			),
			expected: true,
		},
		// redis error tests
		{
			name:     "redis.ErrClosed",
			err:      redis.ErrClosed,
			expected: true,
		},
		{
			name:     "redis.ErrPoolExhausted",
			err:      redis.ErrPoolExhausted,
			expected: true,
		},
		{
			name:     "redis.ErrPoolTimeout",
			err:      redis.ErrPoolTimeout,
			expected: true,
		},
		{
			name:     "wrapped redis.ErrPoolTimeout",
			err:      errors.Join(errors.New("redis operation failed"), redis.ErrPoolTimeout),
			expected: true,
		},
		// Fallback string pattern tests
		{
			name: "pgx dial error pattern",
			err: errors.New(
				"failed to connect to `user=postgres database=`: 10.96.122.184:5432: dial error: dial tcp 10.96.122.184:5432: connect: operation not permitted",
			),
			expected: true,
		},
		{
			name:     "pgx connection refused pattern",
			err:      errors.New("failed to connect: connection refused"),
			expected: true,
		},
		{
			name:     "i/o timeout pattern",
			err:      errors.New("read tcp 127.0.0.1:5432: i/o timeout"),
			expected: true,
		},
		{
			name:     "no such host pattern",
			err:      errors.New("dial tcp: lookup unknown-host: no such host"),
			expected: true,
		},
		{
			name:     "unrelated error should not match",
			err:      errors.New("syntax error in SQL query"),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := common.IsDBConnectionError(tc.err)
			if result != tc.expected {
				t.Errorf("IsDBConnectionError(%v) = %v, expected %v", tc.err, result, tc.expected)
			}
		})
	}
}
