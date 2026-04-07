package api

import (
	"sync"
	"time"
)

type terminalRenderMode string

const (
	terminalRenderModeLive     terminalRenderMode = "live"
	terminalRenderModeSnapshot terminalRenderMode = "snapshot"

	defaultTerminalSnapshotInterval = 50 * time.Millisecond
	minTerminalSnapshotInterval     = 33 * time.Millisecond
	maxTerminalSnapshotInterval     = 10 * time.Second
)

type terminalConnectionRenderState struct {
	mu                         sync.RWMutex
	mode                       terminalRenderMode
	snapshotInterval           time.Duration
	snapshotCompressionEnabled bool
	notifyCh                   chan struct{}
}

func newTerminalConnectionRenderState() *terminalConnectionRenderState {
	return &terminalConnectionRenderState{
		mode:                       terminalRenderModeLive,
		snapshotInterval:           defaultTerminalSnapshotInterval,
		snapshotCompressionEnabled: true,
		notifyCh:                   make(chan struct{}, 1),
	}
}

func (s *terminalConnectionRenderState) Mode() terminalRenderMode {
	if s == nil {
		return terminalRenderModeLive
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

func (s *terminalConnectionRenderState) SnapshotConfig() (terminalRenderMode, time.Duration, bool) {
	if s == nil {
		return terminalRenderModeLive, defaultTerminalSnapshotInterval, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode, s.snapshotInterval, s.snapshotCompressionEnabled
}

func (s *terminalConnectionRenderState) NotifyC() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.notifyCh
}

func (s *terminalConnectionRenderState) Update(
	mode string,
	snapshotIntervalMs int,
	snapshotCompressionEnabled bool,
) (
	previous terminalRenderMode,
	current terminalRenderMode,
	interval time.Duration,
	compressionEnabled bool,
	changed bool,
) {
	if s == nil {
		normalizedMode := normalizeTerminalRenderMode(mode)
		return terminalRenderModeLive, normalizedMode, normalizeSnapshotInterval(snapshotIntervalMs), snapshotCompressionEnabled, true
	}

	nextMode := normalizeTerminalRenderMode(mode)
	nextInterval := normalizeSnapshotInterval(snapshotIntervalMs)

	s.mu.Lock()
	previous = s.mode
	previousInterval := s.snapshotInterval
	previousCompressionEnabled := s.snapshotCompressionEnabled
	s.mode = nextMode
	s.snapshotInterval = nextInterval
	s.snapshotCompressionEnabled = snapshotCompressionEnabled
	current = s.mode
	interval = s.snapshotInterval
	compressionEnabled = s.snapshotCompressionEnabled
	changed = previous != current || previousInterval != interval || previousCompressionEnabled != compressionEnabled
	s.mu.Unlock()

	if changed {
		select {
		case s.notifyCh <- struct{}{}:
		default:
		}
	}

	return previous, current, interval, compressionEnabled, changed
}

func normalizeTerminalRenderMode(value string) terminalRenderMode {
	switch value {
	case string(terminalRenderModeSnapshot):
		return terminalRenderModeSnapshot
	default:
		return terminalRenderModeLive
	}
}

func normalizeSnapshotInterval(value int) time.Duration {
	if value <= 0 {
		return defaultTerminalSnapshotInterval
	}
	duration := time.Duration(value) * time.Millisecond
	if duration < minTerminalSnapshotInterval {
		return minTerminalSnapshotInterval
	}
	if duration > maxTerminalSnapshotInterval {
		return maxTerminalSnapshotInterval
	}
	return duration
}

func snapshotIntervalMilliseconds(duration time.Duration) int {
	if duration <= 0 {
		duration = defaultTerminalSnapshotInterval
	}
	return int(duration / time.Millisecond)
}

func normalizeSnapshotCompression(value string) bool {
	return value != "none"
}
