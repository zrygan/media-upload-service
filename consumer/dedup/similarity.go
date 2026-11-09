package dedup

// frameDistance returns the Hamming distance of two frame hashes, which
// is the number of bits that differ between them.
func frameDistance(a, b uint64) int {
	// TODO: implement
	panic("frameDistance: not implemented")
}

// isInformative returns false if the frame hash is from a near-uniform
// frame, like a black screen. These hash to almost the same value, so
// they should not count as matching frames.
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

// isDuplicate returns true if the signatures a and b are of the same
// video, based on the number of frames that matched in bestAlignment.
func isDuplicate(a, b []uint64) bool {
	// TODO: implement
	panic("isDuplicate: not implemented")
}
