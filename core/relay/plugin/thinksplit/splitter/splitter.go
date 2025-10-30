package splitter

import "slices"

type Splitter struct {
	heads           [][]byte
	tails           [][]byte
	buffer          []byte
	state           int
	partialTailPos  []int
	kmpNexts        [][]int
	impossibleHeads []bool // Track which heads cannot match (inverted logic)
	longestHeadLen  int    // Cache the longest head length
}

func NewSplitter(heads, tails [][]byte) *Splitter {
	kmpNexts := make([][]int, len(tails))
	for i, tail := range tails {
		kmpNexts[i] = computeKMPNext(tail)
	}

	// Find the longest head pattern length
	longestHeadLen := 0
	for _, head := range heads {
		if len(head) > longestHeadLen {
			longestHeadLen = len(head)
		}
	}

	return &Splitter{
		heads:          heads,
		tails:          tails,
		kmpNexts:       kmpNexts,
		partialTailPos: make([]int, len(tails)),
		impossibleHeads: make(
			[]bool,
			len(heads),
		), // Defaults to false (all heads initially possible)
		longestHeadLen: longestHeadLen,
	}
}

func computeKMPNext(pattern []byte) []int {
	n := len(pattern)

	next := make([]int, n)
	if n == 0 {
		return next
	}

	next[0] = 0
	for i := 1; i < n; i++ {
		j := next[i-1]
		for j > 0 && pattern[i] != pattern[j] {
			j = next[j-1]
		}

		if pattern[i] == pattern[j] {
			j++
		}

		next[i] = j
	}

	return next
}

func (s *Splitter) Process(data []byte) ([]byte, []byte) {
	if len(data) == 0 {
		return nil, nil
	}

	switch s.state {
	case 0:
		s.buffer = append(s.buffer, data...)
		bufLen := len(s.buffer)

		headMatched := false
		headMatchLen := 0
		anyPossibleHead := false

		// Check all heads in a single pass
		for i, head := range s.heads {
			// Skip if this head has already been ruled out
			if s.impossibleHeads[i] {
				continue
			}

			headLen := len(head)

			// Check for complete match
			if bufLen >= headLen {
				if slices.Equal(s.buffer[:headLen], head) {
					headMatched = true
					headMatchLen = headLen
					break
				}
				// Mark this head as impossible to match
				s.impossibleHeads[i] = true
			} else {
				// Check for partial match (potential match)
				matchLen := bufLen
				if slices.Equal(s.buffer[:matchLen], head[:matchLen]) {
					anyPossibleHead = true
				} else {
					// Mark this head as impossible to match
					s.impossibleHeads[i] = true
				}
			}
		}

		if headMatched {
			// Head found, move to seeking tail
			s.state = 1

			s.buffer = s.buffer[headMatchLen:]
			if len(s.buffer) == 0 {
				return nil, nil
			}

			return s.processSeekTail()
		}

		if anyPossibleHead {
			// Need more data to determine if a head matches
			return nil, nil
		}

		// No head matches and no partial match possible, move to done state
		s.state = 2
		remaining := s.buffer
		s.buffer = nil

		return nil, remaining

	case 1:
		s.buffer = append(s.buffer, data...)
		return s.processSeekTail()

	default:
		return nil, data
	}
}

func (s *Splitter) processSeekTail() ([]byte, []byte) {
	data := s.buffer

	// Check for each tail pattern
	for tailIdx, tail := range s.tails {
		j := s.partialTailPos[tailIdx]
		tailLen := len(tail)
		kmpNext := s.kmpNexts[tailIdx]

		for i := range data {
			for j > 0 && data[i] != tail[j] {
				j = kmpNext[j-1]
			}

			if data[i] == tail[j] {
				j++
				if j == tailLen {
					end := max(i-tailLen+1, 0)

					result := data[:end]
					remaining := data[i+1:]
					s.buffer = nil
					s.state = 2

					return result, remaining
				}
			}
		}

		// Update partial match position for this tail
		s.partialTailPos[tailIdx] = j
	}

	// Determine how much of the buffer we can safely return
	minSafePos := len(data)
	for _, pos := range s.partialTailPos {
		if pos > 0 {
			// We have a partial match for this tail
			tailMatchLen := pos

			safePos := len(data) - tailMatchLen
			if safePos < minSafePos {
				minSafePos = safePos
			}
		}
	}

	if minSafePos <= 0 {
		// We can't safely return anything yet
		return nil, nil
	}

	result := data[:minSafePos]
	s.buffer = data[minSafePos:]

	return result, nil
}
