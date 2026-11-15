package httpauth

import (
	"net/http"
	"net/netip"
	"strings"
)

// Forwarded returns a ClientIP that reads X-Forwarded-For, but only when the
// request arrived from one of the trusted proxies. From anyone else the
// header is ignored, because anyone can send it.
//
// It walks the header from the right, skipping addresses that belong to
// trusted proxies, and returns the first one that does not: the nearest
// address no trusted proxy vouches for. A malformed entry makes it fall back
// to the peer address rather than guess.
func Forwarded(trusted []netip.Prefix) ClientIP {
	return func(r *http.Request) netip.Addr {
		peer := RemoteAddr(r)
		if !within(trusted, peer) {
			return peer
		}
		hops := forwardedFor(r)
		for i := len(hops) - 1; i >= 0; i-- {
			addr, err := netip.ParseAddr(hops[i])
			if err != nil {
				return peer
			}
			if !within(trusted, addr) {
				return addr
			}
		}
		return peer
	}
}

func within(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

// forwardedFor lists the addresses in every X-Forwarded-For header, in order.
func forwardedFor(r *http.Request) []string {
	var hops []string
	for _, value := range r.Header.Values("X-Forwarded-For") {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				hops = append(hops, part)
			}
		}
	}
	return hops
}
