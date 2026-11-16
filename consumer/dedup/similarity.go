package dedup

import "math/bits"

const (
	// The maximum Hamming distance between two 64-bit frame hashes for
	// the two frames to count as a match.
	maxFrameDistance = 10
	// The minimum number of 1 bits (and 0 bits) a frame hash must have
	// to not be treated as a near-uniform frame.
	minInformativeBits = 4
	// The minimum fraction of informative frames that must match in
	// bestAlignment for two signatures to count as the same video.
	minMatchRatio = 0.7
)

// frameDistance returns the number of bits that differ between two frame
// hashes (the Hamming distance).
func frameDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// isInformative returns false for a near-uniform frame, like a black
// screen. Those all hash alike, so they shouldn't count as matches.
func isInformative(hash uint64) bool {
	ones := bits.OnesCount64(hash)
	return ones >= minInformativeBits && ones <= 64-minInformativeBits
}

// bestAlignment slides the signatures a and b against each other and
// returns the largest number of matching frames across all offsets.
func bestAlignment(a, b []uint64) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	best := 0
	for offset := -(len(b) - 1); offset <= len(a)-1; offset++ {
		startA := max(0, offset)
		startB := max(0, -offset)
		overlap := min(len(a)-startA, len(b)-startB)

		if overlap <= best {
			continue
		}

		matches := 0
		for i := range overlap {
			fa := a[startA+i]
			fb := b[startB+i]
			if isInformative(fa) && isInformative(fb) && frameDistance(fa, fb) <= maxFrameDistance {
				matches++
			}
		}

		if matches > best {
			best = matches
		}
	}

	return best
}

// isDuplicate reports whether a and b are the same video, based on the
// matching frames from bestAlignment.
func isDuplicate(a, b []uint64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}

	infA := 0
	for _, h := range a {
		if isInformative(h) {
			infA++
		}
	}

	infB := 0
	for _, h := range b {
		if isInformative(h) {
			infB++
		}
	}

	if infA == 0 || infB == 0 {
		return false
	}

	matches := bestAlignment(a, b)
	return float64(matches)/float64(max(infA, infB)) >= minMatchRatio
}
