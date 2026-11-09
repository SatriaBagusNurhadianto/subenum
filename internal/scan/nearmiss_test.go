package scan

import (
	"context"
	"testing"
	"time"

	"github.com/TMHSDigital/subenum/internal/dns"
	"github.com/TMHSDigital/subenum/internal/dnstest"
)

// TestIsWildcardAnswer covers every branch of the near-miss revalidation
// added for #48 (#169), against the in-repo DNS server. Every name under
// example.com answers like a rotating wildcard pool: A 192.0.2.2 and
// 192.0.2.3, while the fingerprint from preflight only saw 192.0.2.1 and
// 192.0.2.2.
func TestIsWildcardAnswer(t *testing.T) {
	srv := dnstest.Start(t, func(dnstest.Query) dnstest.Reply {
		return dnstest.Reply{A: []string{"192.0.2.2", "192.0.2.3"}}
	})
	a := func(ips ...string) []dns.Record {
		var out []dns.Record
		for _, ip := range ips {
			out = append(out, dns.Record{Type: "A", Value: ip})
		}
		return out
	}
	live := Config{
		Resolver: dns.NewResolver(2*time.Second, srv.Addr), DNSServer: srv.Addr,
		Timeout: 2 * time.Second, Attempts: 1, Types: []string{"A"},
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	for _, c := range []struct {
		name    string
		cfg     Config
		ctx     context.Context
		records []dns.Record
		want    bool
		probes  int64 // queries the check may send
		grows   bool  // the fingerprint learns the pool's third address
	}{
		{name: "covered", cfg: live, records: a("192.0.2.1"), want: true},
		{name: "nothing shared: a real name, no probes", cfg: live, records: a("198.51.100.7"), want: false},
		{name: "near miss the pool explains", cfg: live, records: a("192.0.2.2", "192.0.2.3"), want: true, probes: 1, grows: true},
		{name: "real name sharing one record", cfg: live, records: a("192.0.2.2", "198.51.100.7"), want: false, probes: nearMissProbes, grows: true},
		{name: "cancelled mid-check keeps the result", cfg: live, ctx: canceled, records: a("192.0.2.2", "198.51.100.7"), want: false},
		{name: "simulated runs never probe", cfg: Config{Simulate: true}, records: a("192.0.2.2", "198.51.100.7"), want: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			ctx := c.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			fp := newFingerprint(a("192.0.2.1", "192.0.2.2"))
			before := srv.Queries()
			if got := isWildcardAnswer(ctx, c.cfg, fp, "host.example.com", c.records); got != c.want {
				t.Errorf("isWildcardAnswer = %v, want %v", got, c.want)
			}
			if sent := srv.Queries() - before; sent != c.probes {
				t.Errorf("%d probe queries, want %d", sent, c.probes)
			}
			if grew := len(fp.records()) > 2; grew != c.grows {
				t.Errorf("fingerprint %v, grown = %v, want %v", fp.records(), grew, c.grows)
			}
		})
	}
}
