package httpapi

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// SecurityOptions configures protections that matter once the API is publicly reachable.
type SecurityOptions struct {
	// AllowedOrigins lists extra browser origins (beyond the site itself) allowed to call the
	// API cross-origin, e.g. "https://my-frontend.example". "*" allows any origin.
	AllowedOrigins []string
	// TrustProxyHeaders takes the client IP from X-Forwarded-For. Only safe behind a proxy
	// that sets that header itself, such as Vercel's edge.
	TrustProxyHeaders bool
	// WritesPerMinute caps POST/DELETE requests per client IP; 0 disables the limit.
	WritesPerMinute int
}

// ConfigureSecurity applies opts. The write limit is kept in Redis when available, so it holds
// across every instance of a horizontally scaled deployment, and in memory otherwise.
// Must be called before Router.
func (s *Server) ConfigureSecurity(opts SecurityOptions) {
	s.allowedOrigins = opts.AllowedOrigins
	s.trustProxy = opts.TrustProxyHeaders
	s.writeLimiter = nil
	if opts.WritesPerMinute > 0 {
		if s.redisClient != nil {
			s.writeLimiter = &redisLimiter{client: s.redisClient, limit: opts.WritesPerMinute}
		} else {
			s.writeLimiter = newMemoryLimiter(opts.WritesPerMinute)
		}
	}
}

// cors adds CORS headers only for origins in the allowlist. The dashboard is served from the
// same origin as the API, so it never needs them; by default nothing else gets them either.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originListed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// guardWrites protects state-changing requests. It rejects ones a browser sent on behalf of
// a page on another site (CSRF): browsers always attach Origin to cross-site POST/DELETE, and
// even an allowlist-free CORS policy doesn't stop a "simple" cross-site POST from executing.
// Requests without Origin (curl, server-to-server) aren't browser-driven and pass. Then it
// applies the per-client write rate limit.
func (s *Server) guardWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" && !s.sameSite(origin, r) && !s.originListed(origin) {
			writeError(w, http.StatusForbidden, "cross-site requests are not allowed")
			return
		}

		if s.writeLimiter != nil {
			if ok, retryAfter := s.writeLimiter.Allow(r.Context(), s.clientIP(r)); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
				writeError(w, http.StatusTooManyRequests, "too many changes from this address; try again in a minute")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originListed(origin string) bool {
	for _, o := range s.allowedOrigins {
		if o == "*" || strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

// sameSite reports whether origin is this site itself. Behind a trusted proxy the public
// host may only be in X-Forwarded-Host, so that's accepted too.
func (s *Server) sameSite(origin string, r *http.Request) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false // includes the opaque "null" origin of sandboxed pages
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	return s.trustProxy && strings.EqualFold(u.Host, r.Header.Get("X-Forwarded-Host"))
}

func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimiter counts one hit for key and reports whether it's within the limit; when it
// isn't, retryAfter says how long until the window resets.
type rateLimiter interface {
	Allow(ctx context.Context, key string) (ok bool, retryAfter time.Duration)
}

// redisLimiter is a fixed one-minute window per key, shared by every instance.
type redisLimiter struct {
	client *redis.Client
	limit  int
}

func (l *redisLimiter) Allow(ctx context.Context, key string) (bool, time.Duration) {
	now := time.Now()
	window := now.Truncate(time.Minute)
	redisKey := fmt.Sprintf("ratelimit:writes:%s:%d", key, window.Unix())

	n, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		// Fail open: an unavailable cache shouldn't take writes down with it.
		log.Printf("httpapi: rate limit check failed, allowing request: %v", err)
		return true, 0
	}
	if n == 1 {
		l.client.Expire(ctx, redisKey, 2*time.Minute)
	}
	if n > int64(l.limit) {
		return false, window.Add(time.Minute).Sub(now)
	}
	return true, 0
}

// memoryLimiter is the single-instance fallback when Redis isn't configured.
type memoryLimiter struct {
	limit int
	mu    sync.Mutex
	hits  map[string]memoryWindow
}

type memoryWindow struct {
	start time.Time
	count int
}

func newMemoryLimiter(limit int) *memoryLimiter {
	return &memoryLimiter{limit: limit, hits: map[string]memoryWindow{}}
}

func (l *memoryLimiter) Allow(_ context.Context, key string) (bool, time.Duration) {
	now := time.Now()
	window := now.Truncate(time.Minute)

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.hits) > 10000 { // bound memory: drop windows that have already ended
		for k, w := range l.hits {
			if w.start.Before(window) {
				delete(l.hits, k)
			}
		}
	}

	w := l.hits[key]
	if !w.start.Equal(window) {
		w = memoryWindow{start: window}
	}
	w.count++
	l.hits[key] = w

	if w.count > l.limit {
		return false, window.Add(time.Minute).Sub(now)
	}
	return true, 0
}
