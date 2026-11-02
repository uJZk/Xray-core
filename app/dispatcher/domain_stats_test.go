package dispatcher

import (
	"context"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	applog "github.com/xtls/xray-core/app/log"
	appstats "github.com/xtls/xray-core/app/stats"
	"github.com/xtls/xray-core/common/buf"
	clog "github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport"
)

func TestTrafficCSVIndependentOfStats(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "no_stats", true: "stats_reset"}[enabled], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "traffic.csv")
			logger, err := applog.New(context.Background(), &applog.Config{TrafficLogPath: path})
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Close()
			var sm stats.Manager = stats.NoopManager{}
			if enabled {
				sm, _ = appstats.NewManager(context.Background(), &appstats.Config{})
			}
			link := &transport.Link{Reader: buf.NewReader(strings.NewReader("hello")), Writer: buf.Discard}
			ctx := wrapDomainStats(context.Background(), sm, link, net.TCPDestination(net.DomainAddress("Example.COM."), 443))
			clog.TrafficFromContext(ctx).SetIP("192.0.2.1")
			mb, err := link.Reader.ReadMultiBuffer()
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}
			buf.ReleaseMulti(mb)
			if err := link.Writer.WriteMultiBuffer(buf.MergeBytes(nil, []byte("reply"))); err != nil {
				t.Fatal(err)
			}
			if enabled {
				sm.GetCounter("domain>>>example.com>>>traffic>>>downlink").Set(0)
			}
			// Same hook used when Vision splice bypasses the writer.
			link.Writer.(*SizeStatWriter).AddCount(3)
			if err := logger.Close(); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			rows, err := csv.NewReader(f).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 || strings.Join(rows[1][2:], ",") != "example.com,192.0.2.1,5,8" {
				t.Fatal(rows)
			}
		})
	}
}

func TestDomainStats(t *testing.T) {
	sm, _ := appstats.NewManager(context.Background(), &appstats.Config{})
	for _, domain := range []string{"Example.COM.", "example.com"} {
		link := &transport.Link{Reader: buf.NewReader(strings.NewReader("hello")), Writer: buf.Discard}
		wrapDomainStats(context.Background(), sm, link, net.TCPDestination(net.DomainAddress(domain), 443))
		mb, err := link.Reader.ReadMultiBuffer()
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		buf.ReleaseMulti(mb)
		if err := link.Writer.WriteMultiBuffer(buf.MergeBytes(nil, []byte("reply"))); err != nil {
			t.Fatal(err)
		}
	}
	for _, direction := range []string{"uplink", "downlink"} {
		c := sm.GetCounter("domain>>>example.com>>>traffic>>>" + direction)
		if c == nil || c.Value() != 10 {
			t.Fatalf("%s: want 10, got %v", direction, c)
		}
	}
}

func TestIPStats(t *testing.T) {
	sm, _ := appstats.NewManager(context.Background(), &appstats.Config{})
	for _, address := range []string{"192.0.2.1", "2001:db8::1"} {
		for _, port := range []net.Port{80, 443} {
			link := &transport.Link{Reader: buf.NewReader(strings.NewReader("hello")), Writer: buf.Discard}
			wrapDomainStats(context.Background(), sm, link, net.TCPDestination(net.ParseAddress(address), port))
			mb, err := link.Reader.ReadMultiBuffer()
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}
			buf.ReleaseMulti(mb)
			if err := link.Writer.WriteMultiBuffer(buf.MergeBytes(nil, []byte("reply"))); err != nil {
				t.Fatal(err)
			}
		}
		for _, direction := range []string{"uplink", "downlink"} {
			c := sm.GetCounter("domain>>>" + address + ">>>traffic>>>" + direction)
			if c == nil || c.Value() != 10 {
				t.Fatalf("%s %s: want 10, got %v", address, direction, c)
			}
		}
	}
}
