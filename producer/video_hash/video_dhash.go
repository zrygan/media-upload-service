package videohash

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/zrygan/media-upload-service/util/ffmpeg"
)

const (
	// The dimensions of each frame after ffmpeg shrinks it. The width
	// is one more than the height since dHash compares each pixel with
	// its right neighbor, which gives 8 comparisons for each of the 8
	// rows (or 64 bits).
	frameWidth  = 9
	frameHeight = 8
	// The size of one frame in the output of ffmpeg, each pixel is one
	// grayscale byte.
	frameBytes = frameWidth * frameHeight
)

// VideoSignature returns the perceptual signature of the video, which is
// the dHash of each frame sampled from the video at one frame per second.
// The hashes stay in time order, so the hash at index i describes the
// video at around i seconds.
//
// The signature is sent to the consumer in the upload metadata, and the
// consumer compares it against the signatures of the other videos.
//
// NOTE: Unlike SHA-256, a re-encoded copy of a video has a signature that
// is close to the original, but usually not identical.
func VideoSignature(ctx context.Context, video *os.File) ([]uint64, error) {
	raw, err := extractFrameRTI(ctx, video, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to extract frames from %s: %w", video.Name(), err)
	}

	signature := make([]uint64, 0, len(raw)/frameBytes)

	for frame := range slices.Chunk(raw, frameBytes) {
		signature = append(signature, dhash(frame))
	}

	return signature, nil
}

// extractFrameRTI extracts frames from the video at Regular Time Intervals
// and returns them as one slice of bytes, where every frameBytes bytes is
// one frame. The pixel at (x, y) of a frame is frame[y*frameWidth+x].
//
// This function also scales and greyscales the frames it extracts.
func extractFrameRTI(ctx context.Context, video *os.File, fps int) ([]byte, error) {
	raw, err := ffmpeg.Run(
		ctx,
		"-v", "error",
		"-i", video.Name(),
		"-vf", fmt.Sprintf("fps=%d,scale=%d:%d:flags=area,format=gray", fps, frameWidth, frameHeight),
		"-f", "rawvideo",
		"pipe:1",
	)
	if err != nil {
		return nil, err
	}

	if len(raw) == 0 || len(raw)%frameBytes != 0 {
		return nil, fmt.Errorf("ffmpeg returned %d bytes, expected a non-zero multiple of %d", len(raw), frameBytes)
	}

	return raw, nil
}

// dhash returns the 64-bit difference hash of one frame. The frame must
// be frameBytes long, see extractFrameRTI.
//
// Each bit is the result of comparing a pixel with its right neighbor: 1
// if the right pixel is brighter, and 0 otherwise.
//
// NOTE: We compare neighboring pixels and not their exact values since
// the values change when a video is re-encoded or brightened, but which
// pixel is brighter usually does not.
func dhash(frame []byte) uint64 {
	var hash uint64

	for row := range frameHeight {
		for col := range frameWidth - 1 {
			hash <<= 1
			if frame[row*frameWidth+col] < frame[row*frameWidth+col+1] {
				hash |= 1
			}
		}
	}

	return hash
}
