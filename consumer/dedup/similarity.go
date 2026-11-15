package dedup

// frameDistance returns the number of bits that differ between two frame
// hashes (the Hamming distance).
func frameDistance(a, b uint64) int {
	// TODO: implement
	panic("frameDistance: not implemented")
}

// isInformative returns false for a near-uniform frame, like a black
// screen. Those all hash alike, so they shouldn't count as matches.
func isInformative(hash uint64) bool {
	// TODO: implement
	panic("isInformative: not implemented")
}

// bestAlignment slides the signatures a and b against each other and
// returns the largest number of matching frames across all offsets.
func bestAlignment(a, b []uint64) int {
	// TODO: implement
	panic("bestAlignment: not implemented")
}

// isDuplicate reports whether a and b are the same video, based on the
// matching frames from bestAlignment.
func isDuplicate(a, b []uint64) bool {
	// TODO: implement
	panic("isDuplicate: not implemented")
}
