package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// CacheEntry represents a cached HTTP response
type CacheEntry struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	Expiration time.Time
}

// CacheMiddleware provides HTTP response caching
type CacheMiddleware struct {
	cache     map[string]*CacheEntry
	mutex     sync.RWMutex
	config    *config.CacheConfig
	log       logger.Logger
	size      int
	evictList []string // List of cache keys ordered by access time
}

// NewCacheMiddleware creates a new cache middleware
func NewCacheMiddleware(config *config.CacheConfig, log logger.Logger) *CacheMiddleware {
	return &CacheMiddleware{
		cache:     make(map[string]*CacheEntry),
		config:    config,
		log:       log,
		evictList: make([]string, 0),
	}
}

// PurgeCache handles cache purge requests
func (c *CacheMiddleware) PurgeCache(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method for purging
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Get the path pattern to purge from query parameter
	pathPattern := r.URL.Query().Get("path")

	beforeCount := len(c.cache)
	if pathPattern != "" {
		// Purge specific path pattern
		for key := range c.cache {
			if strings.Contains(key, pathPattern) {
				delete(c.cache, key)
			}
		}
	} else {
		// Purge all cache if no path specified
		c.cache = make(map[string]*CacheEntry)
	}
	afterCount := len(c.cache)
	purgedCount := beforeCount - afterCount

	c.log.Info("Cache purged",
		logger.String("path_pattern", pathPattern),
		logger.Int("purged_entries", purgedCount),
		logger.Int("remaining_entries", afterCount),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"success":           true,
		"purged_entries":    purgedCount,
		"remaining_entries": afterCount,
	}
	json.NewEncoder(w).Encode(response)
}

// RegisterPurgeEndpoint registers the cache purge endpoint
func (c *CacheMiddleware) RegisterPurgeEndpoint(router http.Handler) http.Handler {
	if !c.config.Enabled || c.config.PurgeEndpoint == "" {
		return router
	}

	handler := http.NewServeMux()

	// Copy all requests to the original router
	handler.Handle("/", router)

	// Add the purge endpoint
	handler.HandleFunc(c.config.PurgeEndpoint, c.PurgeCache)

	c.log.Info("Registered cache purge endpoint",
		logger.String("endpoint", c.config.PurgeEndpoint),
	)

	return handler
}

// Cache middleware caches responses for GET requests
func (c *CacheMiddleware) Cache(next http.Handler, route config.Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip caching if not enabled for this route or if it's not a GET request
		if !c.shouldCache(r, route) {
			next.ServeHTTP(w, r)
			return
		}

		// Generate cache key from request
		key := c.generateCacheKey(r)

		// Try to get from cache
		entry := c.getFromCache(key)
		if entry != nil {
			c.log.Debug("Cache hit",
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method),
				logger.String("key", key),
			)
			c.serveFromCache(w, entry)
			return
		}

		// If not in cache, capture the response
		c.log.Debug("Cache miss",
			logger.String("path", r.URL.Path),
			logger.String("method", r.Method),
			logger.String("key", key),
		)

		// Create a buffer to store the response
		buf := &bytes.Buffer{}

		// Create a custom response writer to capture the response
		crw := &cachingResponseWriter{
			ResponseWriter: w,
			buffer:         buf,
			statusCode:     http.StatusOK,
			headers:        make(http.Header),
		}

		// Process the request
		next.ServeHTTP(crw, r)

		// Don't cache error responses
		if crw.statusCode >= 400 {
			return
		}

		// Determine TTL for cache entry
		ttl := c.getTTL(r, crw.headers, route)
		if ttl <= 0 {
			return
		}

		// Store in cache
		c.storeInCache(key, crw.statusCode, buf.Bytes(), crw.headers, ttl)
	})
}

// shouldCache determines if a request should be cached
func (c *CacheMiddleware) shouldCache(r *http.Request, route config.Route) bool {
	// Check if cache is globally disabled
	if !c.config.Enabled {
		return false
	}

	// Only cache enabled routes
	if route.Cache == nil || !route.Cache.Enabled {
		return false
	}

	// Only cache GET requests
	if r.Method != http.MethodGet {
		return false
	}

	// Don't cache if Cache-Control: no-cache or no-store
	cacheControl := r.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-cache") || strings.Contains(cacheControl, "no-store") {
		return false
	}

	// Don't cache authenticated requests unless specified
	if !route.Cache.CacheAuthenticated && (r.Header.Get("Authorization") != "" || r.Header.Get("x-api-key") != "") {
		return false
	}

	return true
}

// generateCacheKey creates a unique key for the cache entry
func (c *CacheMiddleware) generateCacheKey(r *http.Request) string {
	// Basic key components
	key := r.Method + ":" + r.URL.Path + ":" + r.URL.RawQuery

	// Add host if vhost-based routing is used
	if c.config.IncludeHost {
		key = r.Host + ":" + key
	}

	// Add certain headers to the key if configured
	for _, header := range c.config.VaryHeaders {
		if value := r.Header.Get(header); value != "" {
			key += ":" + header + "=" + value
		}
	}

	// Hash the key to keep it a reasonable length
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

// getFromCache retrieves a value from the cache
func (c *CacheMiddleware) getFromCache(key string) *CacheEntry {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil
	}

	// Check if entry has expired
	if time.Now().After(entry.Expiration) {
		// Expired entry, remove it
		go c.removeFromCache(key)
		return nil
	}

	return entry
}

// removeFromCache removes a value from the cache
func (c *CacheMiddleware) removeFromCache(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.cache, key)
}

// storeInCache stores a value in the cache
func (c *CacheMiddleware) storeInCache(key string, statusCode int, body []byte, headers http.Header, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if we need to evict entries
	if c.config.MaxSize > 0 && len(c.cache) >= c.config.MaxSize {
		// Evict oldest entries
		for _, oldKey := range c.evictList[:len(c.evictList)/2] {
			delete(c.cache, oldKey)
		}
		c.evictList = c.evictList[len(c.evictList)/2:]
		c.log.Info("Cache eviction performed",
			logger.Int("evicted_count", len(c.evictList)/2),
			logger.Int("remaining_entries", len(c.cache)),
		)
	}

	// Create a copy of the headers
	headersCopy := make(http.Header)
	for k, v := range headers {
		headersCopy[k] = v
	}

	// Add cache-related headers
	headersCopy.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(ttl.Seconds())))
	headersCopy.Set("Age", "0")
	headersCopy.Set("X-Cache-TTL", fmt.Sprintf("%d", int(ttl.Seconds())))

	// Create a cache entry
	entry := &CacheEntry{
		StatusCode: statusCode,
		Body:       body,
		Headers:    headersCopy,
		Expiration: time.Now().Add(ttl),
	}

	// Store in cache and update eviction list
	c.cache[key] = entry
	c.evictList = append(c.evictList, key)

	// Set up automatic expiration
	time.AfterFunc(ttl, func() {
		c.removeFromCache(key)
	})
}

// serveFromCache serves a cached response
func (c *CacheMiddleware) serveFromCache(w http.ResponseWriter, entry *CacheEntry) {
	// Calculate age of the cache entry
	ttlStr := entry.Headers.Get("X-Cache-TTL")
	ttl, _ := strconv.Atoi(ttlStr)
	age := int(time.Since(entry.Expiration.Add(-time.Duration(ttl) * time.Second)).Seconds())

	// Copy headers from cached response
	for k, v := range entry.Headers {
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}

	// Update cache-related headers
	w.Header().Set("Age", strconv.Itoa(age))
	w.Header().Set("X-Cache", "HIT")

	// Set status code and write body
	w.WriteHeader(entry.StatusCode)
	w.Write(entry.Body)
}

// getTTL determines the TTL for a cache entry
func (c *CacheMiddleware) getTTL(r *http.Request, headers http.Header, route config.Route) time.Duration {
	// Default TTL from route config
	ttl := time.Duration(route.Cache.TTL) * time.Second

	// Check for Cache-Control: max-age
	cacheControl := headers.Get("Cache-Control")
	if strings.Contains(cacheControl, "max-age=") {
		parts := strings.Split(cacheControl, "max-age=")
		if len(parts) > 1 {
			maxAge := strings.Split(parts[1], ",")[0]
			if seconds, err := strconv.Atoi(maxAge); err == nil {
				return time.Duration(seconds) * time.Second
			}
		}
	}

	// Check for Expires header
	expires := headers.Get("Expires")
	if expires != "" {
		if expTime, err := time.Parse(time.RFC1123, expires); err == nil {
			ttl = expTime.Sub(time.Now())
		}
	}

	// If TTL is negative or zero, don't cache
	if ttl <= 0 {
		return 0
	}

	// Apply maximum TTL if configured
	if c.config.MaxTTL > 0 && ttl > time.Duration(c.config.MaxTTL)*time.Second {
		ttl = time.Duration(c.config.MaxTTL) * time.Second
	}

	return ttl
}

// cachingResponseWriter captures the response for caching
type cachingResponseWriter struct {
	http.ResponseWriter
	buffer     *bytes.Buffer
	statusCode int
	headers    http.Header
}

// WriteHeader captures the status code
func (crw *cachingResponseWriter) WriteHeader(statusCode int) {
	crw.statusCode = statusCode
	crw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body
func (crw *cachingResponseWriter) Write(b []byte) (int, error) {
	crw.buffer.Write(b)
	return crw.ResponseWriter.Write(b)
}

// Header captures the response headers
func (crw *cachingResponseWriter) Header() http.Header {
	h := crw.ResponseWriter.Header()

	// Copy headers to our internal storage
	for k, v := range h {
		crw.headers[k] = v
	}

	return h
}
