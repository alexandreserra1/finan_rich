package proxy

import (
	"container/heap"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const maxBurstKeys = 10000

type tokenBucket struct {
	mu       sync.Mutex
	now      func() time.Time
	last     time.Time
	capacity time.Duration
	interval time.Duration
	maxKeys  int
	buckets  map[string]*bucket
	expiry   bucketHeap
}

type bucket struct {
	key             string
	credit          time.Duration
	updated, fullAt time.Time
	index           int
}

type bucketHeap []*bucket

func (entries bucketHeap) Len() int { return len(entries) }
func (entries bucketHeap) Less(left, right int) bool {
	return entries[left].fullAt.Before(entries[right].fullAt)
}
func (entries bucketHeap) Swap(left, right int) {
	entries[left], entries[right] = entries[right], entries[left]
	entries[left].index, entries[right].index = left, right
}
func (entries *bucketHeap) Push(value any) {
	entry := value.(*bucket)
	entry.index = len(*entries)
	*entries = append(*entries, entry)
}
func (entries *bucketHeap) Pop() any {
	last := len(*entries) - 1
	entry := (*entries)[last]
	(*entries)[last] = nil
	*entries = (*entries)[:last]
	return entry
}

func newTokenBucket(burst int, interval time.Duration, maxKeys int, now func() time.Time) *tokenBucket {
	return &tokenBucket{now: now, capacity: time.Duration(burst) * interval, interval: interval, maxKeys: maxKeys, buckets: make(map[string]*bucket)}
}

func (throttle *tokenBucket) allow(ip string) time.Duration {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	now := throttle.now()
	if now.Before(throttle.last) {
		now = throttle.last
	}
	throttle.last = now
	ip = limitIP(ip)
	entry := throttle.buckets[ip]
	if entry == nil {
		if len(throttle.buckets) >= throttle.maxKeys {
			oldest := throttle.expiry[0]
			if now.Before(oldest.fullAt) {
				return oldest.fullAt.Sub(now)
			}
			heap.Pop(&throttle.expiry)
			delete(throttle.buckets, oldest.key)
		}
		entry = &bucket{key: ip, credit: throttle.capacity, updated: now, fullAt: now}
		throttle.buckets[ip] = entry
		heap.Push(&throttle.expiry, entry)
	}
	entry.credit = min(throttle.capacity, entry.credit+min(throttle.capacity, now.Sub(entry.updated)))
	entry.updated = now
	var wait time.Duration
	if entry.credit >= throttle.interval {
		entry.credit -= throttle.interval
	} else {
		wait = throttle.interval - entry.credit
	}
	entry.fullAt = now.Add(throttle.capacity - entry.credit)
	heap.Fix(&throttle.expiry, entry.index)
	return wait
}

func limitIP(ip string) string {
	if parsed := net.ParseIP(ip); parsed != nil {
		if parsed.To4() == nil {
			return parsed.Mask(net.CIDRMask(64, 128)).String() + "/64" // IPv6: a casa tem o /64 inteiro
		}
	}
	return ip
}

func rateLimited(writer http.ResponseWriter, message string, wait time.Duration) {
	seconds := wait / time.Second
	if wait%time.Second != 0 {
		seconds++
	}
	writer.Header().Set("Retry-After", strconv.FormatInt(int64(max(seconds, 1)), 10))
	writer.Header().Set("Cache-Control", "no-store")
	fail(writer, http.StatusTooManyRequests, message)
}
