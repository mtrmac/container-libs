package copy

import (
	"bytes"
	"io"
	"testing"
	"testing/synctest"
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"go.podman.io/image/v5/types"
)

// consumeNonblocking drains all currently buffered events
// from a channel and returns them. It never blocks.
func consumeNonblocking(channel <-chan types.ProgressProperties) []types.ProgressProperties {
	var result []types.ProgressProperties
	for {
		select {
		case p := <-channel:
			result = append(result, p)
		default:
			return result
		}
	}
}

// TestNewProgressReporter verifies that constructing a reporter
// signals a new artifact event.
func TestNewProgressReporter(t *testing.T) {
	const (
		artifactDigest = digest.Digest("sha256:7173b809ca12ec5dee4506cd86be934c4596dd234ee82c0662eac04a8c2c71dc")
		artifactSize   = 1024
	)
	channel := make(chan types.ProgressProperties, 10)
	artifact := types.BlobInfo{Digest: artifactDigest, Size: artifactSize}

	r := newChannelProgressReporter(channel, time.Second, artifact)
	assert.NotNil(t, r, "newChannelProgressReporter should return a progress reporter")

	// Verify only a new artifact event was received.
	events := consumeNonblocking(channel)
	assert.Equal(t, []types.ProgressProperties{{
		Event:    types.ProgressEventNewArtifact,
		Artifact: types.BlobInfo{Digest: artifactDigest, Size: artifactSize},
	}}, events, "constructor should send exactly one new artifact event")
}

// TestProgressReporterReportRead verifies that a read event is sent
// after the interval elapses and not before.
func TestProgressReporterReportRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			artifactDigest = digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")
			artifactSize   = 2048
		)
		channel := make(chan types.ProgressProperties, 10)
		artifact := types.BlobInfo{Digest: artifactDigest, Size: artifactSize}
		interval := 5 * time.Second

		// Create a reporter and consume the new artifact event.
		r := newChannelProgressReporter(channel, interval, artifact)
		assert.NotEmpty(t, consumeNonblocking(channel), "should send new artifact event")

		// Verify that before the interval elapses: no event is sent.
		r.reportRead(5)
		assert.Empty(t, consumeNonblocking(channel), "should not send before interval")

		// Verify that after the interval: read event is sent.
		time.Sleep(2 * interval)
		r.reportRead(10)
		events := consumeNonblocking(channel)
		assert.Equal(t, []types.ProgressProperties{{
			Event:        types.ProgressEventRead,
			Artifact:     types.BlobInfo{Digest: artifactDigest, Size: artifactSize},
			Offset:       15,
			OffsetUpdate: 15,
		}}, events, "should send a read event after interval elapses")
	})
}

// TestProgressReporterReportDone verifies that a done event
// includes the accumulated offset and the not-yet-reported offset update.
func TestProgressReporterReportDone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			artifactDigest = digest.Digest("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
			artifactSize   = 4096
		)
		channel := make(chan types.ProgressProperties, 10)
		artifact := types.BlobInfo{Digest: artifactDigest, Size: artifactSize}
		interval := 5 * time.Second

		// Create a reporter and consume the new artifact event.
		r := newChannelProgressReporter(channel, interval, artifact)
		assert.NotEmpty(t, consumeNonblocking(channel), "should send new artifact event")

		// Simulate progress with three reported reads.
		const (
			read1 = 30
			read2 = 20
			read3 = 15
		)
		r.reportRead(read1)
		assert.Empty(t, consumeNonblocking(channel), "should not send before interval")
		time.Sleep(2 * interval)
		r.reportRead(read2)
		assert.NotEmpty(t, consumeNonblocking(channel), "should send after interval elapses")
		r.reportRead(read3)

		// Verify that the done event includes the
		// accumulated offset from all three reads
		// and the OffsetUpdate is the final read
		// that happened before the update interval
		// elapsed.
		r.reportSuccess()
		events := consumeNonblocking(channel)
		assert.Equal(t, []types.ProgressProperties{{
			Event:        types.ProgressEventDone,
			Artifact:     types.BlobInfo{Digest: artifactDigest, Size: artifactSize},
			Offset:       read1 + read2 + read3,
			OffsetUpdate: read3,
		}}, events, "should send a done event with accumulated offsets")
	})
}

// TestProgressReporterReset verifies that reset does not report
// negative progress and suppresses reports until the offset
// exceeds the previously reported high-water mark.
func TestProgressReporterReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			artifactDigest = digest.Digest("sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae")
			artifactSize   = 8192
		)
		channel := make(chan types.ProgressProperties, 10)
		artifact := types.BlobInfo{Digest: artifactDigest, Size: artifactSize}
		interval := 5 * time.Second

		// Create a reporter and consume the new artifact event.
		r := newChannelProgressReporter(channel, interval, artifact)
		assert.NotEmpty(t, consumeNonblocking(channel), "should send new artifact event")

		// reportRead(10): no event before interval.
		r.reportRead(10)
		assert.Empty(t, consumeNonblocking(channel))

		// After interval, reportRead(10): reports +20=20.
		time.Sleep(2 * interval)
		r.reportRead(10)
		events := consumeNonblocking(channel)
		assert.Equal(t, []types.ProgressProperties{{
			Event:        types.ProgressEventRead,
			Artifact:     types.BlobInfo{Digest: artifactDigest, Size: artifactSize},
			Offset:       20,
			OffsetUpdate: 20,
		}}, events)

		// reportRead(10): no event (interval not elapsed).
		r.reportRead(10)
		assert.Empty(t, consumeNonblocking(channel))

		// reset: no event sent.
		r.reset()
		assert.Empty(t, consumeNonblocking(channel), "reset should not send an event")

		// After interval, reportRead(15): nothing (15 < 20 already reported).
		time.Sleep(2 * interval)
		r.reportRead(15)
		assert.Empty(t, consumeNonblocking(channel), "should not report below high-water mark")

		// After interval, reportRead(10): reports +5=25.
		time.Sleep(2 * interval)
		r.reportRead(10)
		events = consumeNonblocking(channel)
		assert.Equal(t, []types.ProgressProperties{{
			Event:        types.ProgressEventRead,
			Artifact:     types.BlobInfo{Digest: artifactDigest, Size: artifactSize},
			Offset:       25,
			OffsetUpdate: 5,
		}}, events)
	})
}

// TestProgressReporterResetIntervalNotElapsed verifies that reportSuccess
// reports accumulated bytes above the high-water mark
// when called immediately after reportRead without waiting for the interval.
func TestProgressReporterResetIntervalNotElapsed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			artifactDigest = digest.Digest("sha256:fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9")
			artifactSize   = 16384
		)
		channel := make(chan types.ProgressProperties, 10)
		artifact := types.BlobInfo{Digest: artifactDigest, Size: artifactSize}
		interval := 5 * time.Second

		// Create a reporter and consume the new artifact event.
		r := newChannelProgressReporter(channel, interval, artifact)
		assert.NotEmpty(t, consumeNonblocking(channel), "should send new artifact event")

		// Accumulate to 20.
		r.reportRead(10)
		time.Sleep(2 * interval)
		r.reportRead(10)
		assert.NotEmpty(t, consumeNonblocking(channel), "should send read event")

		// Reset, accumulate above the high-water mark and immediately report success.
		r.reset()
		r.reportRead(25)
		r.reportSuccess()
		events := consumeNonblocking(channel)

		// Verify that the accumulated offset is reported together with the done event.
		assert.Equal(t, []types.ProgressProperties{{
			Event:        types.ProgressEventDone,
			Artifact:     types.BlobInfo{Digest: artifactDigest, Size: artifactSize},
			Offset:       25,
			OffsetUpdate: 5,
		}}, events)
	})
}

// TestNoopProgressReporter verifies that
// a noopProgressReporter can be called without panicking.
func TestNoopProgressReporter(t *testing.T) {
	r := &noopProgressReporter{}

	// Make sure that no progressReporter method panics.
	r.reportRead(100)
	r.reset()
	r.reportRead(50)
	r.reportSuccess()
}

func newSUT(
	t *testing.T,
	reader io.Reader,
	duration time.Duration,
	channel chan types.ProgressProperties,
) *progressReader {
	artifact := types.BlobInfo{Size: 10}

	reporter := newChannelProgressReporter(channel, duration, artifact)
	res := <-channel
	assert.Equal(t, res.Event, types.ProgressEventNewArtifact)
	assert.Equal(t, res.Artifact, artifact)

	return newProgressReader(reader, reporter)
}

func TestNewProgressReader(t *testing.T) {
	// Given
	channel := make(chan types.ProgressProperties, 1)
	sut := newSUT(t, nil, time.Second, channel)
	assert.NotNil(t, sut)

	// When/Then
	go func() {
		res := <-channel
		assert.Equal(t, res.Event, types.ProgressEventDone)
	}()
	sut.reportSuccess()
}

func TestReadWithoutEvent(t *testing.T) {
	// Given
	channel := make(chan types.ProgressProperties, 1)
	reader := bytes.NewReader([]byte{0, 1, 2})
	sut := newSUT(t, reader, time.Second, channel)
	assert.NotNil(t, sut)

	// When
	b := []byte{0, 1, 2, 3, 4}
	read, err := reader.Read(b)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, read, 3)
}

func TestReadWithEvent(t *testing.T) {
	// Given
	channel := make(chan types.ProgressProperties, 1)
	reader := bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6})
	sut := newSUT(t, reader, time.Nanosecond, channel)
	assert.NotNil(t, sut)
	b := []byte{0, 1, 2, 3, 4}

	// When/Then
	go func() {
		res := <-channel
		assert.Equal(t, res.Event, types.ProgressEventRead)
		assert.Equal(t, res.Offset, uint64(5))
		assert.Equal(t, res.OffsetUpdate, uint64(5))
	}()
	read, err := reader.Read(b)
	assert.Equal(t, read, 5)
	assert.Nil(t, err)
}
