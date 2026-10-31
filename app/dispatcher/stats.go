package dispatcher

import (
	"strings"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport"
)

type SizeStatWriter struct {
	Counter stats.Counter
	Writer  buf.Writer
}

func (w *SizeStatWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	w.Counter.Add(int64(mb.Len()))
	return w.Writer.WriteMultiBuffer(mb)
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
}

func (r *SizeStatReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	mb, err := r.Reader.ReadMultiBuffer()
	r.Counter.Add(int64(mb.Len()))
	return mb, err
}

func (r *SizeStatReader) Interrupt() { common.Interrupt(r.Reader) }

// wrapDomainStats uses the same counters and API as upstream user statistics.
// IP-only destinations use their canonical IP string as the key.
func wrapDomainStats(sm stats.Manager, link *transport.Link, destination net.Destination) {
	var domain string
	if destination.Address.Family().IsDomain() {
		domain = strings.TrimSuffix(strings.ToLower(destination.Address.Domain()), ".")
	} else {
		domain = destination.Address.IP().String()
	}
	if domain == "" {
		return
	}
	if c, _ := sm.GetOrRegisterCounter("domain>>>" + domain + ">>>traffic>>>uplink"); c != nil {
		link.Reader = &SizeStatReader{Reader: link.Reader, Counter: c}
	}
	if c, _ := sm.GetOrRegisterCounter("domain>>>" + domain + ">>>traffic>>>downlink"); c != nil {
		link.Writer = &SizeStatWriter{Writer: link.Writer, Counter: c}
	}
}
