package netif

import (
	"net"
	"syscall"
	"testing"
)

func TestParseIPAddr(t *testing.T) {
	tests := []struct {
		name   string
		family byte
		prefix int
		attrs  []syscall.NetlinkRouteAttr
		want   string
		fail   bool
	}{
		{name: "ipv4 local preferred", family: syscall.AF_INET, prefix: 24, attrs: []syscall.NetlinkRouteAttr{{Attr: syscall.RtAttr{Type: syscall.IFA_ADDRESS}, Value: []byte{192, 0, 2, 9}}, {Attr: syscall.RtAttr{Type: syscall.IFA_LOCAL}, Value: []byte{192, 0, 2, 1}}}, want: "192.0.2.1/24"},
		{name: "ipv6", family: syscall.AF_INET6, prefix: 64, attrs: []syscall.NetlinkRouteAttr{{Attr: syscall.RtAttr{Type: syscall.IFA_ADDRESS}, Value: net.ParseIP("2001:db8::1").To16()}}, want: "2001:db8::1/64"},
		{name: "short address", family: syscall.AF_INET, prefix: 24, attrs: []syscall.NetlinkRouteAttr{{Attr: syscall.RtAttr{Type: syscall.IFA_ADDRESS}, Value: []byte{1, 2, 3}}}, fail: true},
		{name: "missing address", family: syscall.AF_INET6, prefix: 64, fail: true},
		{name: "invalid prefix", family: syscall.AF_INET, prefix: 33, fail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := parseIPAddr(tt.family, tt.prefix, tt.attrs)
			if tt.fail {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if addr.String() != tt.want {
				t.Fatalf("got %s, want %s", addr, tt.want)
			}
		})
	}
}

func TestMalformedRouteAttributes(t *testing.T) {
	for _, data := range [][]byte{
		{0, 0, 0, 0},
		{8, 0, 1, 0, 1},
	} {
		message := syscall.NetlinkMessage{
			Header: syscall.NlMsghdr{Type: syscall.RTM_NEWADDR},
			Data:   append(make([]byte, syscall.SizeofIfAddrmsg), data...),
		}
		if _, err := syscall.ParseNetlinkRouteAttr(&message); err == nil {
			t.Fatalf("accepted malformed attribute %v", data)
		}
	}
}
