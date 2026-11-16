package dedup

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestFrameDistance(t *testing.T) {
	if got := frameDistance(0xaaaa_aaaa_aaaa_aaaa, 0xaaaa_aaaa_aaaa_aaaa); got != 0 {
		t.Errorf("frameDistance(same) = %d, want 0", got)
	}
	if got := frameDistance(0, ^uint64(0)); got != 64 {
		t.Errorf("frameDistance(0, ^0) = %d, want 64", got)
	}
	if got := frameDistance(0b1010, 0b0110); got != 2 {
		t.Errorf("frameDistance(0b1010, 0b0110) = %d, want 2", got)
	}
}

func TestIsInformative(t *testing.T) {
	if isInformative(0) {
		t.Error("isInformative(0) = true, want false for uniform black/flat frame")
	}
	if isInformative(^uint64(0)) {
		t.Error("isInformative(^0) = true, want false for uniform gradient frame")
	}
	if isInformative(0b111) {
		t.Error("isInformative(3 bits) = true, want false")
	}
	if !isInformative(0x0f0f_0f0f_0f0f_0f0f) {
		t.Error("isInformative(32 bits) = false, want true")
	}
}

func TestBestAlignmentAndIsDuplicate(t *testing.T) {
	orig := []uint64{
		0x1234_5678_9abc_def0,
		0x0fed_cba9_8765_4321,
		0xaaaa_5555_aaaa_5555,
		0x3333_cccc_3333_cccc,
		0xdead_beef_cafe_babe,
	}

	// Re-encoded copy with a few bits flipped per frame (distance <= 10)
	// and shifted by 1 frame at the beginning (a black intro frame added).
	reencoded := []uint64{
		0x0000_0000_0000_0000, // uninformative black frame
		0x1234_5678_9abc_def0 ^ 0b111,
		0x0fed_cba9_8765_4321 ^ 0b10101,
		0xaaaa_5555_aaaa_5555 ^ 0b110011,
		0x3333_cccc_3333_cccc ^ 0b1,
		0xdead_beef_cafe_babe ^ 0b1111,
	}

	if got := bestAlignment(orig, reencoded); got != 5 {
		t.Errorf("bestAlignment(orig, reencoded) = %d, want 5", got)
	}
	if !isDuplicate(orig, reencoded) {
		t.Error("isDuplicate(orig, reencoded) = false, want true")
	}

	// Unrelated video should not match.
	other := []uint64{
		^orig[0],
		^orig[1],
		^orig[2],
		^orig[3],
		^orig[4],
	}
	if isDuplicate(orig, other) {
		t.Error("isDuplicate(orig, other) = true, want false")
	}

	// Two all-black signatures should not match via perceptual similarity.
	black := []uint64{0, 0, 0, 0, 0}
	if isDuplicate(black, black) {
		t.Error("isDuplicate(black, black) = true, want false")
	}
}

func TestIndexCheckAndInsertAndRemove(t *testing.T) {
	idx := NewIndex()
	sig := []uint64{
		0x1234_5678_9abc_def0,
		0x0fed_cba9_8765_4321,
		0xaaaa_5555_aaaa_5555,
		0x3333_cccc_3333_cccc,
	}

	// First insert should not be a duplicate.
	if idx.CheckAndInsert("hash1", sig) {
		t.Fatal("first CheckAndInsert returned true, want false")
	}

	// Exact same ID should be detected as a duplicate.
	if !idx.CheckAndInsert("hash1", sig) {
		t.Fatal("second CheckAndInsert with same ID returned false, want true")
	}

	// Different ID with perceptually similar signature should be detected as a duplicate.
	reencoded := []uint64{
		sig[0] ^ 0b11,
		sig[1] ^ 0b101,
		sig[2] ^ 0b1,
		sig[3] ^ 0b111,
	}
	if !idx.CheckAndInsert("hash2", reencoded) {
		t.Fatal("CheckAndInsert with perceptual duplicate returned false, want true")
	}

	// After removing hash1, inserting it again should succeed.
	idx.Remove("hash1")
	if idx.CheckAndInsert("hash1", sig) {
		t.Fatal("CheckAndInsert after Remove returned true, want false")
	}
}

func TestIndexConcurrentCheckAndInsert(t *testing.T) {
	idx := NewIndex()
	sig := []uint64{
		0x1234_5678_9abc_def0,
		0x0fed_cba9_8765_4321,
		0xaaaa_5555_aaaa_5555,
		0x3333_cccc_3333_cccc,
	}

	const goroutines = 32
	var accepted atomic.Int32
	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			if !idx.CheckAndInsert("same-hash", sig) {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()

	if got := accepted.Load(); got != 1 {
		t.Errorf("concurrent CheckAndInsert accepted %d uploads, want 1", got)
	}
}
