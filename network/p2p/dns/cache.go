package dns

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/mempool"
)

// DefaultTimeToLive is the default duration a dns result is cached.
const (
	// DefaultTimeToLive is the default duration a dns result is cached.
	DefaultTimeToLive = 5 * time.Minute
	DefaultCacheSize  = 10e3
	cacheEntryExists  = true
	cacheEntryFresh   = true // TTL yet has not reached
)

// cache is a ttl-based cache for dns entries
type cache struct {
	sync.RWMutex

	ttl    time.Duration // time-to-live for cache entry
	dCache mempool.DNSCache
	logger zerolog.Logger
}

func newCache(logger zerolog.Logger, dnsCache mempool.DNSCache) *cache {
	return &cache{
		ttl:    DefaultTimeToLive,
		dCache: dnsCache,
		logger: logger.With().Str("component", "dns-cache").Logger(),
	}
}

// resolveIPCache resolves the domain through the cache if it is available.
// First boolean variable determines whether the domain exists in the cache.
// Second boolean variable determines whether the domain cache is fresh, i.e., TTL has not yet reached.
func (c *cache) resolveIPCache(domain string) ([]net.IPAddr, bool, bool) {
	c.RLock()
	defer c.RUnlock()

	record, ok := c.dCache.GetDomainIp(domain)
	currentTimeStamp := runtimeNano()
	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryFresh
	}
	c.logger.Trace().
		Str("domain", domain).
		Str("address", fmt.Sprintf("%v", record.Addresses)).
		Int64("record_timestamp", record.Timestamp).
		Int64("current_timestamp", currentTimeStamp).
		Msg("dns record retrieved")

	if time.Duration(currentTimeStamp-record.Timestamp) > c.ttl {
		// exists but expired
		return record.Addresses, cacheEntryExists, !cacheEntryFresh
	}

	// exists and fresh
	return record.Addresses, cacheEntryExists, cacheEntryFresh
}

// resolveIPCache resolves the txt through the cache if it is available.
// First boolean variable determines whether the txt record exists in the cache.
// Second boolean variable determines whether the txt record cache is fresh, i.e., TTL has not yet reached.
func (c *cache) resolveTXTCache(txt string) ([]string, bool, bool) {
	c.RLock()
	defer c.RUnlock()

	record, ok := c.dCache.GetTxtRecord(txt)
	currentTimeStamp := runtimeNano()
	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryFresh
	}
	c.logger.Trace().
		Str("txt", txt).
		Str("address", fmt.Sprintf("%v", record.Record)).
		Int64("record_timestamp", record.Timestamp).
		Int64("current_timestamp", currentTimeStamp).
		Msg("dns record retrieved")

	if time.Duration(currentTimeStamp-record.Timestamp) > c.ttl {
		// exists but expired
		return record.Record, cacheEntryExists, !cacheEntryFresh
	}

	// exists and fresh
	return record.Record, cacheEntryExists, cacheEntryFresh
}

// updateIPCache updates the cache entry for the domain.
func (c *cache) updateIPCache(domain string, addr []net.IPAddr) {
	c.Lock()
	defer c.Unlock()

	timestamp := runtimeNano()
	removed := c.dCache.RemoveIp(domain)
	added := c.dCache.PutDomainIp(domain, addr, runtimeNano())

	ipSize, txtSize := c.dCache.Size()
	c.logger.Trace().
		Str("domain", domain).
		Str("address", fmt.Sprintf("%v", addr)).
		Bool("old_entry_removed", removed).
		Bool("new_entry_added", added).
		Int64("timestamp", timestamp).
		Uint("ip_size", ipSize).
		Uint("txt_size", txtSize).
		Msg("dns cache updated")
}

// updateTXTCache updates the cache entry for the txt record.
func (c *cache) updateTXTCache(txt string, record []string) {
	c.Lock()
	defer c.Unlock()

	timestamp := runtimeNano()
	removed := c.dCache.RemoveTxt(txt)
	added := c.dCache.PutTxtRecord(txt, record, runtimeNano())

	ipSize, txtSize := c.dCache.Size()
	c.logger.Trace().
		Str("txt", txt).
		Strs("record", record).
		Bool("old_entry_removed", removed).
		Bool("new_entry_added", added).
		Int64("timestamp", timestamp).
		Uint("ip_size", ipSize).
		Uint("txt_size", txtSize).
		Msg("dns cache updated")
}

// invalidateIPCacheEntry atomically invalidates ip cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateIPCacheEntry(domain string) bool {
	c.Lock()
	defer c.Unlock()

	return c.dCache.RemoveIp(domain)
}

// invalidateTXTCacheEntry atomically invalidates txt cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateTXTCacheEntry(txt string) bool {
	c.Lock()
	defer c.Unlock()

	return c.dCache.RemoveTxt(txt)
}
