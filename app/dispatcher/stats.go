package dispatcher

import (
	"context"
	"strings"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport"
)

type SizeStatWriter struct {
	Counter stats.Counter
	Writer  buf.Writer
	Traffic func(int64)
}

func (w *SizeStatWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	w.AddCount(int64(mb.Len()))
	return w.Writer.WriteMultiBuffer(mb)
}

// AddCount also accounts for bytes copied directly by Vision splice.
func (w *SizeStatWriter) AddCount(n int64) {
	if w.Counter != nil {
		w.Counter.Add(n)
	}
	if w.Traffic != nil {
		w.Traffic(n)
	}
}

func (w *SizeStatWriter) Close() error {
	return common.Close(w.Writer)
}

func (w *SizeStatWriter) Interrupt() {
	common.Interrupt(w.Writer)
}

// SizeStatReader counts payload after sniffing, including any cached first bytes.
type SizeStatReader struct {
	Reader  buf.Reader
	Counter stats.Counter
	Traffic func(int64)
}

func (r *SizeStatReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	mb, err := r.Reader.ReadMultiBuffer()
	if r.Counter != nil {
		r.Counter.Add(int64(mb.Len()))
	}
	if r.Traffic != nil {
		r.Traffic(int64(mb.Len()))
	}
	return mb, err
}

func (r *SizeStatReader) Interrupt() { common.Interrupt(r.Reader) }

// wrapDomainStats uses the same counters and API as upstream user statistics.
// IP-only destinations use their canonical IP string as the key.
func wrapDomainStats(ctx context.Context, sm stats.Manager, link *transport.Link, destination net.Destination) context.Context {
	var domain string
	if destination.Address.Family().IsDomain() {
		domain = strings.TrimSuffix(strings.ToLower(destination.Address.Domain()), ".")
	} else {
		domain = destination.Address.IP().String()
	}
	if domain == "" {
		return ctx
	}
	ctx, traffic := log.TrackTraffic(ctx, domain, destination.Network.SystemString())
	if c, _ := sm.GetOrRegisterCounter("domain>>>" + domain + ">>>traffic>>>uplink"); c != nil || traffic != nil {
		reader := &SizeStatReader{Reader: link.Reader, Counter: c}
		if traffic != nil {
			reader.Traffic = traffic.AddUplink
		}
		link.Reader = reader
	}
	if c, _ := sm.GetOrRegisterCounter("domain>>>" + domain + ">>>traffic>>>downlink"); c != nil || traffic != nil {
		writer := &SizeStatWriter{Writer: link.Writer, Counter: c}
		if traffic != nil {
			writer.Traffic = traffic.AddDownlink
		}
		link.Writer = writer
	}
	return ctx
}
