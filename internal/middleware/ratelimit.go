package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Best-effort per-IP fixed-window rate limiter held in memory. It's per warm
// Lambda instance (not global), so it pairs with reserved concurrency: a small
// instance count bounds total throughput, this throttles each instance.
//
// The window is generous on purpose — at a real venue many players share one
// public IP (same wifi), so the limit must clear legitimate polling traffic
// while still cutting off floods.
type limiter struct {
	mu      sync.Mutex
	hits    map[string]*window
	max     int
	windowD time.Duration
}

type window struct {
	count int
	start time.Time
}

func newLimiter(max int, d time.Duration) *limiter {
	return &limiter{hits: make(map[string]*window), max: max, windowD: d}
}

func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// unbounded-growth guard: evict only EXPIRED windows, never wipe the whole
	// map — a full reset would hand every active client a fresh burst at once
	// (and let a many-IP flood bypass the limiter by repeatedly triggering it).
	if len(l.hits) > 20000 {
		for k, v := range l.hits {
			if now.Sub(v.start) > l.windowD {
				delete(l.hits, k)
			}
		}
	}

	w, ok := l.hits[ip]
	if !ok || now.Sub(w.start) > l.windowD {
		l.hits[ip] = &window{count: 1, start: now}
		return true
	}
	w.count++
	return w.count <= l.max
}

// RateLimit caps requests per client IP. 300 requests / 10s clears heavy
// shared-wifi polling but blocks scripted floods.
func RateLimit() gin.HandlerFunc {
	return rateLimitWith(newLimiter(300, 10*time.Second))
}

// RateLimitStrict is a much tighter per-IP limit for CPU-expensive or
// brute-forceable endpoints (e.g. password verification, which runs bcrypt).
// 20 / 10s is generous for a human typing a gate code but caps bcrypt CPU
// burn and password guessing from a single IP.
func RateLimitStrict() gin.HandlerFunc {
	return rateLimitWith(newLimiter(20, 10*time.Second))
}

func rateLimitWith(l *limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests,
				gin.H{"success": false, "error": "請求太頻繁,請稍後再試"})
			return
		}
		c.Next()
	}
}

// BodyLimit rejects oversized request bodies (default 64KB) before they're read.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge,
				gin.H{"success": false, "error": "request too large"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
