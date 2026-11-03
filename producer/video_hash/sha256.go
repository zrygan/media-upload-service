package videohash

import (
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
)

// SHA256 is a wrapper function that calls the standard libraries
// implementation of the SHA256 (via Sum256) but takes in a video of
// type *os.File instead of an array of bytes.
func SHA256(video *os.File) ([]byte, error) {
	var sum []byte
	hasher := sha256.New()

	if _, err := io.Copy(hasher, io.NewSectionReader(video, 0, math.MaxInt64)); err != nil {
		return sum, fmt.Errorf("SHA256 failed to stream %s: %w", video.Name(), err)
	}

	hasher.Sum(sum[:0])
	return sum, nil
}
