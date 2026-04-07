package terminal

import "time"

const terminalTrafficRecentWindow = 5 * time.Minute

type sessionTrafficBucket struct {
	timestamp       time.Time
	upstreamBytes   uint64
	downstreamBytes uint64
}

type SessionTrafficStats struct {
	UpstreamBytes                  uint64  `json:"upstreamBytes"`
	DownstreamBytes                uint64  `json:"downstreamBytes"`
	TotalBytes                     uint64  `json:"totalBytes"`
	UpstreamRecentBytes            uint64  `json:"upstreamRecentBytes"`
	DownstreamRecentBytes          uint64  `json:"downstreamRecentBytes"`
	TotalRecentBytes               uint64  `json:"totalRecentBytes"`
	UpstreamAvgBytesPerSec         float64 `json:"upstreamAvgBytesPerSec"`
	DownstreamAvgBytesPerSec       float64 `json:"downstreamAvgBytesPerSec"`
	TotalAvgBytesPerSec            float64 `json:"totalAvgBytesPerSec"`
	UpstreamRecentAvgBytesPerSec   float64 `json:"upstreamRecentAvgBytesPerSec"`
	DownstreamRecentAvgBytesPerSec float64 `json:"downstreamRecentAvgBytesPerSec"`
	TotalRecentAvgBytesPerSec      float64 `json:"totalRecentAvgBytesPerSec"`
}

func (s *Session) RecordTraffic(upstreamBytes, downstreamBytes int) {
	if s == nil || (upstreamBytes <= 0 && downstreamBytes <= 0) {
		return
	}

	now := time.Now()
	bucketTime := now.Truncate(time.Second)

	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()

	if upstreamBytes > 0 {
		s.totalUpstreamBytes += uint64(upstreamBytes)
	}
	if downstreamBytes > 0 {
		s.totalDownstreamBytes += uint64(downstreamBytes)
	}

	lastIndex := len(s.trafficBuckets) - 1
	if lastIndex >= 0 && s.trafficBuckets[lastIndex].timestamp.Equal(bucketTime) {
		if upstreamBytes > 0 {
			s.trafficBuckets[lastIndex].upstreamBytes += uint64(upstreamBytes)
		}
		if downstreamBytes > 0 {
			s.trafficBuckets[lastIndex].downstreamBytes += uint64(downstreamBytes)
		}
	} else {
		bucket := sessionTrafficBucket{timestamp: bucketTime}
		if upstreamBytes > 0 {
			bucket.upstreamBytes = uint64(upstreamBytes)
		}
		if downstreamBytes > 0 {
			bucket.downstreamBytes = uint64(downstreamBytes)
		}
		s.trafficBuckets = append(s.trafficBuckets, bucket)
	}

	s.pruneTrafficBucketsLocked(now)
}

func (s *Session) TrafficStatsSnapshot() *SessionTrafficStats {
	if s == nil {
		return nil
	}

	now := time.Now()

	s.mu.RLock()
	createdAt := s.createdAt
	s.mu.RUnlock()

	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()

	s.pruneTrafficBucketsLocked(now)

	totalUpstream := s.totalUpstreamBytes
	totalDownstream := s.totalDownstreamBytes

	recentUpstream := uint64(0)
	recentDownstream := uint64(0)
	cutoff := now.Add(-terminalTrafficRecentWindow)
	for _, bucket := range s.trafficBuckets {
		if bucket.timestamp.Before(cutoff) {
			continue
		}
		recentUpstream += bucket.upstreamBytes
		recentDownstream += bucket.downstreamBytes
	}

	sessionAgeSeconds := now.Sub(createdAt).Seconds()
	if sessionAgeSeconds < 1 {
		sessionAgeSeconds = 1
	}

	recentWindowSeconds := sessionAgeSeconds
	if recentWindowSeconds > terminalTrafficRecentWindow.Seconds() {
		recentWindowSeconds = terminalTrafficRecentWindow.Seconds()
	}
	if recentWindowSeconds < 1 {
		recentWindowSeconds = 1
	}

	return &SessionTrafficStats{
		UpstreamBytes:                  totalUpstream,
		DownstreamBytes:                totalDownstream,
		TotalBytes:                     totalUpstream + totalDownstream,
		UpstreamRecentBytes:            recentUpstream,
		DownstreamRecentBytes:          recentDownstream,
		TotalRecentBytes:               recentUpstream + recentDownstream,
		UpstreamAvgBytesPerSec:         float64(totalUpstream) / sessionAgeSeconds,
		DownstreamAvgBytesPerSec:       float64(totalDownstream) / sessionAgeSeconds,
		TotalAvgBytesPerSec:            float64(totalUpstream+totalDownstream) / sessionAgeSeconds,
		UpstreamRecentAvgBytesPerSec:   float64(recentUpstream) / recentWindowSeconds,
		DownstreamRecentAvgBytesPerSec: float64(recentDownstream) / recentWindowSeconds,
		TotalRecentAvgBytesPerSec:      float64(recentUpstream+recentDownstream) / recentWindowSeconds,
	}
}

func (s *Session) pruneTrafficBucketsLocked(now time.Time) {
	cutoff := now.Add(-terminalTrafficRecentWindow)
	keepFrom := 0
	for keepFrom < len(s.trafficBuckets) {
		if !s.trafficBuckets[keepFrom].timestamp.Before(cutoff) {
			break
		}
		keepFrom++
	}
	if keepFrom == 0 {
		return
	}
	if keepFrom >= len(s.trafficBuckets) {
		s.trafficBuckets = nil
		return
	}
	s.trafficBuckets = append([]sessionTrafficBucket(nil), s.trafficBuckets[keepFrom:]...)
}
