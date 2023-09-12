package cache

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metadata"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// ServeDNS implements the plugin.Handler interface.
func (c *Cache) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	rc := r.Copy() // We potentially modify r, to prevent other plugins from seeing this (r is a pointer), copy r into rc.
	state := request.Request{W: w, Req: rc}
	do := state.Do()
	ad := r.AuthenticatedData

	zone := plugin.Zones(c.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(c.Name(), c.Next, ctx, w, rc)
	}

	now := c.now().UTC()
	server := metrics.WithServer(ctx)

	// On cache refresh, we will just use the DO bit from the incoming query for the refresh since we key our cache
	// with the query DO bit. That means two separate cache items for the query DO bit true or false. In the situation
	// in which upstream doesn't support DNSSEC, the two cache items will effectively be the same. Regardless, any
	// DNSSEC RRs in the response are written to cache with the response.

	fmt.Printf("In handler::ServeDNS. Going to check the cache for our item.\n")
	fmt.Printf("As a reminder, this is our item '%s'\n", rc.String())
	ttl := 0
	i := c.getIgnoreTTL(now, state, server)

	if i == nil {
		crr := &ResponseWriter{ResponseWriter: w, Cache: c, state: state, server: server, do: do, ad: ad,
			nexcept: c.nexcept, pexcept: c.pexcept, wildcardFunc: wildcardFunc(ctx)}
		fmt.Printf("Calling doRefresh SITE 1\n") //okay, this is where we do the question asking?
		return c.doRefresh(ctx, state, crr)
	}
	ttl = i.ttl(now)
	if ttl < 0 {
		// serve stale behavior
		if c.verifyStale {
			crr := &ResponseWriter{ResponseWriter: w, Cache: c, state: state, server: server, do: do}
			cw := newVerifyStaleResponseWriter(crr)
			fmt.Printf("Calling doRefresh SITE 2\n")
			ret, err := c.doRefresh(ctx, state, cw)
			if cw.refreshed {
				return ret, err
			}
		}

		// Adjust the time to get a 0 TTL in the reply built from a stale item.
		now = now.Add(time.Duration(ttl) * time.Second)
		if !c.verifyStale {
			cw := newPrefetchResponseWriter(server, state, c)
			fmt.Printf("Setting up doPrefetch gorouting SITE 1\n")
			go c.doPrefetch(ctx, state, cw, i, now)
		}
		servedStale.WithLabelValues(server, c.zonesMetricLabel, c.viewMetricLabel).Inc()
	} else if c.shouldPrefetch(i, now) {
		cw := newPrefetchResponseWriter(server, state, c)
		fmt.Printf("Setting up doPrefetch gorouting SITE 2\n")
		go c.doPrefetch(ctx, state, cw, i, now)
	}

	if i.wildcard != "" {
		fmt.Printf("Going to call setValueFunc because wildcard is not empty string!\n")
		// Set wildcard source record name to metadata
		metadata.SetValueFunc(ctx, "zone/wildcard", func() string {
			return i.wildcard
		})
	}

	if c.keepttl {
		// If keepttl is enabled we fake the current time to the stored
		// one so that we always get the original TTL
		now = i.stored
	}
	//Okay 'resp' contains the ANSWER.
	resp := i.toMsg(r, now, do, ad)
	fmt.Printf("And here we are before the WriteMsg call. this is still our item: '%s'\n", r.String())
	fmt.Printf("And here is our resp, before the call: '%s'\n", resp.String())
	w.WriteMsg(resp)
	fmt.Printf("And here is our resp, and AFTER the call: '%s'\n", resp.String())
	return dns.RcodeSuccess, nil
}

func wildcardFunc(ctx context.Context) func() string {
	return func() string {
		// Get wildcard source record name from metadata
		if f := metadata.ValueFunc(ctx, "zone/wildcard"); f != nil {
			return f()
		}
		return ""
	}
}

func (c *Cache) doPrefetch(ctx context.Context, state request.Request, cw *ResponseWriter, i *item, now time.Time) {
	cachePrefetches.WithLabelValues(cw.server, c.zonesMetricLabel, c.viewMetricLabel).Inc()
	c.doRefresh(ctx, state, cw)

	// When prefetching we loose the item i, and with it the frequency
	// that we've gathered sofar. See we copy the frequencies info back
	// into the new item that was stored in the cache.
	if i1 := c.exists(state); i1 != nil {
		i1.Freq.Reset(now, i.Freq.Hits())
	}
}

func (c *Cache) doRefresh(ctx context.Context, state request.Request, cw dns.ResponseWriter) (int, error) {
	return plugin.NextOrFailure(c.Name(), c.Next, ctx, cw, state.Req)
}

func (c *Cache) shouldPrefetch(i *item, now time.Time) bool {
	if c.prefetch <= 0 {
		return false
	}
	i.Freq.Update(c.duration, now)
	threshold := int(math.Ceil(float64(c.percentage) / 100 * float64(i.origTTL)))
	return i.Freq.Hits() >= c.prefetch && i.ttl(now) <= threshold
}

// Name implements the Handler interface.
func (c *Cache) Name() string { return "cache" }

// getIgnoreTTL unconditionally returns an item if it exists in the cache.
func (c *Cache) getIgnoreTTL(now time.Time, state request.Request, server string) *item {
	k := hash(state.Name(), state.QType(), state.Do())
	cacheRequests.WithLabelValues(server, c.zonesMetricLabel, c.viewMetricLabel).Inc()

	if i, ok := c.ncache.Get(k); ok {
		itm := i.(*item)
		ttl := itm.ttl(now)
		if itm.matches(state) && (ttl > 0 || (c.staleUpTo > 0 && -ttl < int(c.staleUpTo.Seconds()))) {
			cacheHits.WithLabelValues(server, Denial, c.zonesMetricLabel, c.viewMetricLabel).Inc()
			fmt.Printf("Found item in the ncache. Returning it.\n")
			return i.(*item)
		}
	}
	if i, ok := c.pcache.Get(k); ok {
		itm := i.(*item)
		ttl := itm.ttl(now)
		if itm.matches(state) && (ttl > 0 || (c.staleUpTo > 0 && -ttl < int(c.staleUpTo.Seconds()))) {
			cacheHits.WithLabelValues(server, Success, c.zonesMetricLabel, c.viewMetricLabel).Inc()
			fmt.Printf("Found item in the pcache. Returning it.\n")
			return i.(*item)
		}
	}
	cacheMisses.WithLabelValues(server, c.zonesMetricLabel, c.viewMetricLabel).Inc()
	fmt.Printf("Didn't find the item in the ncache or pcache. Returning nil.\n")
	return nil
}

func (c *Cache) exists(state request.Request) *item {
	k := hash(state.Name(), state.QType(), state.Do())
	if i, ok := c.ncache.Get(k); ok {
		return i.(*item)
	}
	if i, ok := c.pcache.Get(k); ok {
		return i.(*item)
	}
	return nil
}
