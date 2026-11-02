package log

import (
	"context"
	"sync/atomic"
)

// Traffic attributes payload bytes to one target and its known destination IP.
// The recorder is independent of StatsService counters and their resets.
type Traffic struct {
	ip     atomic.Pointer[string]
	record func(ip string, uplink, downlink int64)
}

func NewTrafficCounter(ip string, record func(string, int64, int64)) *Traffic {
	t := &Traffic{record: record}
	t.SetIP(ip)
	return t
}

func (t *Traffic) SetIP(ip string) { t.ip.Store(&ip) }

func (t *Traffic) AddUplink(n int64) {
	if n > 0 {
		t.record(*t.ip.Load(), n, 0)
	}
}

func (t *Traffic) AddDownlink(n int64) {
	if n > 0 {
		t.record(*t.ip.Load(), 0, n)
	}
}

type trafficContextKey struct{}

func TrafficFromContext(ctx context.Context) *Traffic {
	t, _ := ctx.Value(trafficContextKey{}).(*Traffic)
	return t
}

// TrackTraffic binds the active logger's optional recorder to a dispatch.
func TrackTraffic(ctx context.Context, target string) (context.Context, *Traffic) {
	if h := logHandler.Load(); h != nil {
		if recorder, ok := (*h).(interface{ NewTraffic(string) *Traffic }); ok {
			if t := recorder.NewTraffic(target); t != nil {
				return context.WithValue(ctx, trafficContextKey{}, t), t
			}
		}
	}
	return ctx, nil
}
