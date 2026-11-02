package log

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/errors"
	clog "github.com/xtls/xray-core/common/log"
)

var trafficHeader = []string{"interval_start", "interval_end", "target", "target_ip", "uplink_bytes", "downlink_bytes"}

type (
	trafficKey   struct{ target, ip string }
	trafficBytes struct{ up, down int64 }
)

type trafficLogger struct {
	path     string
	mu       sync.Mutex
	flushMu  sync.Mutex
	pending  map[trafficKey]trafficBytes
	start    time.Time
	closed   bool
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
	closeErr error
}

func newTrafficLogger(path string) (*trafficLogger, error) {
	// Validate/create the file now, so bad paths fail startup, not the first tick.
	if err := appendTraffic(path, nil); err != nil {
		return nil, err
	}
	t := &trafficLogger{path: path, pending: make(map[trafficKey]trafficBytes), start: time.Now(), stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(t.done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := t.flush(); err != nil {
					errors.LogWarningInner(context.Background(), err, "failed to append traffic CSV")
				}
			case <-t.stop:
				return
			}
		}
	}()
	return t, nil
}

func (t *trafficLogger) record(key trafficKey, up, down int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	n := t.pending[key]
	n.up += up
	n.down += down
	t.pending[key] = n
}

func (t *trafficLogger) flush() error {
	t.flushMu.Lock()
	defer t.flushMu.Unlock()
	t.mu.Lock()
	start, pending := t.start, t.pending
	end := time.Now()
	t.start, t.pending = end, make(map[trafficKey]trafficBytes)
	t.mu.Unlock()
	if len(pending) == 0 {
		return nil
	}

	keys := make([]trafficKey, 0, len(pending))
	for key := range pending {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].target != keys[j].target {
			return keys[i].target < keys[j].target
		}
		return keys[i].ip < keys[j].ip
	})
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		n := pending[key]
		rows = append(rows, []string{start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano), key.target, key.ip, strconv.FormatInt(n.up, 10), strconv.FormatInt(n.down, 10)})
	}
	if err := appendTraffic(t.path, rows); err != nil {
		// Keep the old interval and add back this batch for the next attempt.
		t.mu.Lock()
		t.start = start
		for key, n := range pending {
			v := t.pending[key]
			v.up += n.up
			v.down += n.down
			t.pending[key] = v
		}
		t.mu.Unlock()
		return err
	}
	return nil
}

func (t *trafficLogger) Close() error {
	t.once.Do(func() {
		close(t.stop)
		<-t.done
		t.mu.Lock()
		t.closed = true
		t.mu.Unlock()
		t.closeErr = t.flush()
	})
	return t.closeErr
}

// Reopen on every batch so rename-based log rotation works. One process must
// own each path. Refuse incompatible headers or a missing trailing newline.
func appendTraffic(path string, rows [][]string) (err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	var payload bytes.Buffer
	w := csv.NewWriter(&payload)
	if info.Size() == 0 {
		if err := w.Write(trafficHeader); err != nil {
			return err
		}
	} else {
		header, err := csv.NewReader(f).Read()
		if err != nil {
			return err
		}
		if len(header) != len(trafficHeader) {
			return fmt.Errorf("incompatible traffic CSV header")
		}
		for i, field := range header {
			if field != trafficHeader[i] {
				return fmt.Errorf("incompatible traffic CSV header")
			}
		}
		var last [1]byte
		if _, err := f.ReadAt(last[:], info.Size()-1); err != nil {
			return err
		}
		if last[0] != '\n' {
			return fmt.Errorf("traffic CSV ends with an incomplete record")
		}
	}
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	data := payload.Bytes()
	if len(data) == 0 {
		return nil
	}
	n, err := f.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	if err != nil {
		// Avoid retrying behind a partially written batch.
		if rollbackErr := f.Truncate(info.Size()); rollbackErr != nil {
			return fmt.Errorf("%w; CSV rollback failed: %v", err, rollbackErr)
		}
	}
	return err
}

// NewTraffic is discovered through the common logger's optional recorder API.
func (g *Instance) NewTraffic(target string) *clog.Traffic {
	g.RLock()
	defer g.RUnlock()
	if !g.active || g.traffic == nil {
		return nil
	}
	ip := ""
	if parsed := net.ParseIP(target); parsed != nil {
		ip = parsed.String()
	}
	return clog.NewTrafficCounter(ip, func(ip string, up, down int64) {
		// Existing connections keep working after the log service is restarted.
		g.RLock()
		defer g.RUnlock()
		if g.active && g.traffic != nil {
			g.traffic.record(trafficKey{target, ip}, up, down)
		}
	})
}
