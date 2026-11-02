package log

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func readTraffic(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestTrafficAppendRestartAndRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.csv")
	g, err := New(context.Background(), &Config{TrafficLogPath: path})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	traffic := g.NewTraffic("example.com")
	traffic.SetIP("192.0.2.1")
	traffic.AddUplink(100)
	traffic.AddDownlink(200)
	if err := g.traffic.flush(); err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	if err := g.Start(); err != nil {
		t.Fatal(err)
	}
	// The same connection's recorder must survive a log service restart.
	traffic.AddDownlink(300)
	if err := g.traffic.flush(); err != nil {
		t.Fatal(err)
	}
	rows := readTraffic(t, path)
	if len(rows) != 3 || !reflect.DeepEqual(rows[0], trafficHeader) {
		t.Fatal(rows)
	}
	if !reflect.DeepEqual(rows[1][2:], []string{"example.com", "192.0.2.1", "100", "200"}) {
		t.Fatal(rows)
	}
	if !reflect.DeepEqual(rows[2][2:], []string{"example.com", "192.0.2.1", "0", "300"}) {
		t.Fatal(rows)
	}
	for _, row := range rows[1:] {
		start, err := time.Parse(time.RFC3339Nano, row[0])
		if err != nil {
			t.Fatal(err)
		}
		end, err := time.Parse(time.RFC3339Nano, row[1])
		if err != nil || end.Before(start) {
			t.Fatal(row)
		}
	}
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	g.NewTraffic("2001:db8::1").AddUplink(7)
	// Close writes remaining bytes even before the first minute elapses.
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	rows = readTraffic(t, path)
	if len(rows) != 2 || !reflect.DeepEqual(rows[1][2:], []string{"2001:db8::1", "2001:db8::1", "7", "0"}) {
		t.Fatal(rows)
	}
}

func TestTrafficFlushRetryAndConcurrentRecording(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.csv")
	tlog, err := newTrafficLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer tlog.Close()
	key := trafficKey{"example.com", "192.0.2.1"}
	start := tlog.start
	tlog.record(key, 10, 20)
	tlog.path = filepath.Join(dir, "missing", "traffic.csv")
	if err := tlog.flush(); err == nil {
		t.Fatal("expected write failure")
	}
	if !tlog.start.Equal(start) {
		t.Fatal("failed batch lost interval start")
	}
	tlog.path = path
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 100 {
				tlog.record(key, 1, 2)
			}
		})
	}
	wg.Wait()
	if err := tlog.flush(); err != nil {
		t.Fatal(err)
	}
	if err := tlog.flush(); err != nil {
		t.Fatal(err)
	}
	rows := readTraffic(t, path)
	if len(rows) != 2 || !reflect.DeepEqual(rows[1][2:], []string{"example.com", "192.0.2.1", "1010", "2020"}) {
		t.Fatal(rows)
	}
}

func TestTrafficRejectsInvalidFiles(t *testing.T) {
	for _, content := range []string{"wrong,header\n", "interval_start,interval_end,target,target_ip,uplink_bytes,downlink_bytes\npartial"} {
		path := filepath.Join(t.TempDir(), "traffic.csv")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if logger, err := newTrafficLogger(path); err == nil {
			logger.Close()
			t.Fatal("expected invalid CSV to fail startup")
		}
		got, _ := os.ReadFile(path)
		if string(got) != content {
			t.Fatal("invalid existing file was modified")
		}
	}
	if logger, err := newTrafficLogger(filepath.Join(t.TempDir(), "missing", "traffic.csv")); err == nil {
		logger.Close()
		t.Fatal("expected invalid path to fail startup")
	}
}
