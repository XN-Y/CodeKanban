package terminal

import (
	"math"
	"testing"
	"time"
)

func TestTrafficStatsSnapshotComputesTotalsAndAverages(t *testing.T) {
	now := time.Now()
	session := &Session{
		createdAt:            now.Add(-10 * time.Second),
		totalUpstreamBytes:   1_000,
		totalDownstreamBytes: 4_000,
		trafficBuckets: []sessionTrafficBucket{
			{
				timestamp:       now.Add(-4 * time.Minute),
				upstreamBytes:   600,
				downstreamBytes: 2_400,
			},
			{
				timestamp:       now.Add(-30 * time.Second),
				upstreamBytes:   200,
				downstreamBytes: 1_000,
			},
		},
	}

	stats := session.TrafficStatsSnapshot()
	if stats == nil {
		t.Fatal("expected stats")
	}
	if stats.UpstreamBytes != 1_000 {
		t.Fatalf("unexpected upstream bytes: %d", stats.UpstreamBytes)
	}
	if stats.DownstreamBytes != 4_000 {
		t.Fatalf("unexpected downstream bytes: %d", stats.DownstreamBytes)
	}
	if stats.TotalBytes != 5_000 {
		t.Fatalf("unexpected total bytes: %d", stats.TotalBytes)
	}
	if stats.UpstreamRecentBytes != 800 {
		t.Fatalf("unexpected recent upstream bytes: %d", stats.UpstreamRecentBytes)
	}
	if stats.DownstreamRecentBytes != 3_400 {
		t.Fatalf("unexpected recent downstream bytes: %d", stats.DownstreamRecentBytes)
	}
	if stats.TotalRecentBytes != 4_200 {
		t.Fatalf("unexpected recent total bytes: %d", stats.TotalRecentBytes)
	}
	if math.Abs(stats.UpstreamAvgBytesPerSec-100) > 0.001 {
		t.Fatalf("unexpected upstream avg: %f", stats.UpstreamAvgBytesPerSec)
	}
	if math.Abs(stats.DownstreamAvgBytesPerSec-400) > 0.001 {
		t.Fatalf("unexpected downstream avg: %f", stats.DownstreamAvgBytesPerSec)
	}
	if math.Abs(stats.UpstreamRecentAvgBytesPerSec-80) > 0.001 {
		t.Fatalf("unexpected upstream recent avg: %f", stats.UpstreamRecentAvgBytesPerSec)
	}
	if math.Abs(stats.DownstreamRecentAvgBytesPerSec-340) > 0.001 {
		t.Fatalf("unexpected downstream recent avg: %f", stats.DownstreamRecentAvgBytesPerSec)
	}
}

func TestRecordTrafficAggregatesBySecondAndPrunesOldBuckets(t *testing.T) {
	session := &Session{
		createdAt: time.Now().Add(-10 * time.Minute),
		trafficBuckets: []sessionTrafficBucket{
			{
				timestamp:       time.Now().Add(-6 * time.Minute),
				upstreamBytes:   10,
				downstreamBytes: 20,
			},
		},
	}

	session.RecordTraffic(100, 300)
	session.RecordTraffic(50, 150)

	session.trafficMu.Lock()
	defer session.trafficMu.Unlock()

	if session.totalUpstreamBytes != 150 {
		t.Fatalf("unexpected total upstream bytes: %d", session.totalUpstreamBytes)
	}
	if session.totalDownstreamBytes != 450 {
		t.Fatalf("unexpected total downstream bytes: %d", session.totalDownstreamBytes)
	}
	if len(session.trafficBuckets) != 1 {
		t.Fatalf("expected old bucket pruned and current bucket merged, got %d buckets", len(session.trafficBuckets))
	}
	if session.trafficBuckets[0].upstreamBytes != 150 {
		t.Fatalf("unexpected merged upstream bytes: %d", session.trafficBuckets[0].upstreamBytes)
	}
	if session.trafficBuckets[0].downstreamBytes != 450 {
		t.Fatalf("unexpected merged downstream bytes: %d", session.trafficBuckets[0].downstreamBytes)
	}
}
