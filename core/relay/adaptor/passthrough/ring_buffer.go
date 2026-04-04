package passthrough

// ringBuffer is a fixed-size circular buffer that always preserves the last N bytes written.
// As bytes are written beyond capacity, the oldest bytes are overwritten.
type ringBuffer struct {
	buf  []byte
	pos  int
	full bool
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{buf: make([]byte, size)}
}

func (rb *ringBuffer) Write(p []byte) (int, error) {
	n := len(p)
	bufLen := len(rb.buf)

	if n >= bufLen {
		// p is larger than the buffer: keep only the last bufLen bytes.
		copy(rb.buf, p[n-bufLen:])
		rb.pos = 0
		rb.full = true

		return n, nil
	}

	remaining := bufLen - rb.pos
	if n <= remaining {
		copy(rb.buf[rb.pos:], p)
		rb.pos += n

		if rb.pos == bufLen {
			rb.pos = 0
			rb.full = true
		}
	} else {
		copy(rb.buf[rb.pos:], p[:remaining])
		copy(rb.buf, p[remaining:])
		rb.pos = n - remaining
		rb.full = true
	}

	return n, nil
}

// Bytes returns the ring buffer contents in chronological (oldest-first) order.
// The returned slice is freshly allocated; the ring buffer is not modified.
func (rb *ringBuffer) Bytes() []byte {
	if !rb.full {
		return rb.buf[:rb.pos]
	}

	// Buffer wrapped: content starts at rb.pos.
	result := make([]byte, len(rb.buf))
	n := copy(result, rb.buf[rb.pos:])
	copy(result[n:], rb.buf[:rb.pos])

	return result
}
