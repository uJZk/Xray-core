package dispatcher

import (
	"context"
	"io"
	"strings"
	"testing"

	appstats "github.com/xtls/xray-core/app/stats"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport"
)

func TestDomainStats(t *testing.T) {
	sm, _ := appstats.NewManager(context.Background(), &appstats.Config{})
	for _, domain := range []string{"Example.COM.", "example.com"} {
		link := &transport.Link{Reader: buf.NewReader(strings.NewReader("hello")), Writer: buf.Discard}
		wrapDomainStats(sm, link, net.TCPDestination(net.DomainAddress(domain), 443))
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
			wrapDomainStats(sm, link, net.TCPDestination(net.ParseAddress(address), port))
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
