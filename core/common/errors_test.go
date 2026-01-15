package common

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
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
				errors.New("database connection failed"),
				&net.OpError{
					Op:  "dial",
					Net: "tcp",
					Err: syscall.ECONNREFUSED,
				},
			),
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsDBConnectionError(tc.err)
			if result != tc.expected {
				t.Errorf("IsDBConnectionError(%v) = %v, expected %v", tc.err, result, tc.expected)
			}
		})
	}
}
