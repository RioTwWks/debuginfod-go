package webapi

import (
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// SecurityOpts — параметры CORS, rate limit и Basic Auth.
type SecurityOpts struct {
	CORSOrigins    []string
	RateLimitRPS   float64
	BasicAuthUser  string
	BasicAuthPass  string
	TLSCertFile    string
	TLSKeyFile     string
	TLSClientCA    string
}

// BasicAuthEnabled возвращает true, если заданы учётные данные.
func (o SecurityOpts) BasicAuthEnabled() bool {
	return o.BasicAuthUser != "" && o.BasicAuthPass != ""
}

// TLSConfigured возвращает true, если заданы сертификат и ключ.
func (o SecurityOpts) TLSConfigured() bool {
	return o.TLSCertFile != "" && o.TLSKeyFile != ""
}

// BuildTLSConfig создаёт TLS-конфигурацию с опциональным mTLS.
func (o SecurityOpts) BuildTLSConfig() (*tls.Config, error) {
	if !o.TLSConfigured() {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if o.TLSClientCA == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(o.TLSClientCA)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, os.ErrInvalid
	}
	cfg.ClientCAs = pool
	cfg.ClientAuth = tls.RequireAndVerifyClientCert
	return cfg, nil
}

// CORSMiddleware добавляет заголовки CORS для публичных инсталляций.
func CORSMiddleware(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}
	allowAll := len(origins) == 1 && origins[0] == "*"
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Zabbix-Token")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	last     time.Time
	rate     float64
	capacity float64
}

func newTokenBucket(rps float64) *tokenBucket {
	capacity := rps
	if capacity < 1 {
		capacity = 1
	}
	return &tokenBucket{
		tokens:   capacity,
		last:     time.Now(),
		rate:     rps,
		capacity: capacity,
	}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type ipLimiter struct {
	sync.Mutex
	buckets map[string]*tokenBucket
	rps     float64
}

func newIPLimiter(rps float64) *ipLimiter {
	if rps <= 0 {
		return nil
	}
	return &ipLimiter{
		buckets: make(map[string]*tokenBucket),
		rps:     rps,
	}
}

func (l *ipLimiter) allow(ip string) bool {
	l.Lock()
	b, ok := l.buckets[ip]
	if !ok {
		b = newTokenBucket(l.rps)
		l.buckets[ip] = b
	}
	l.Unlock()
	return b.allow()
}

// RateLimitMiddleware ограничивает число запросов с одного IP.
func RateLimitMiddleware(rps float64, next http.Handler) http.Handler {
	lim := newIPLimiter(rps)
	if lim == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !lim.allow(clientIP(r)) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// BasicAuthMiddleware защищает эндпоинты Basic Auth (/healthz без auth).
func BasicAuthMiddleware(user, pass string, next http.Handler) http.Handler {
	if user == "" || pass == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="debuginfod"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
