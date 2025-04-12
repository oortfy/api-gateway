package util

import (
	"api-gateway/pkg/logger"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ip2location/ip2location-go/v9"
)

var (
	ip2db      *ip2location.DB
	ip2dbOnce  sync.Once
	ip2dbError error
)

// GetClientIP properly extracts the real client IP from the request,
// handling common proxy and forwarding headers.
func GetClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header (common in reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		// The leftmost IP is the original client
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			return clientIP
		}
	}

	// Check for X-Real-IP header (used by some proxies)
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}

	// Check for Forwarded header (RFC 7239)
	if forwarded := r.Header.Get("Forwarded"); forwarded != "" {
		parts := strings.Split(forwarded, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "for=") {
				// Get the value after 'for='
				value := strings.TrimPrefix(part, "for=")
				// Remove quotes if present
				value = strings.Trim(value, "\"")
				// Handle IPv6 if in brackets [2001:db8:cafe::17]:4711
				if strings.HasPrefix(value, "[") {
					if i := strings.Index(value, "]"); i > 0 {
						return value[1:i]
					}
				}
				// Check for port
				if i := strings.LastIndex(value, ":"); i > 0 {
					return value[:i]
				}
				return value
			}
		}
	}

	// Extract IP from RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If we can't split, use the whole RemoteAddr
		return r.RemoteAddr
	}

	return ip
}

// GetGeoLocation returns country information for the given IP address.
// If the IP is invalid or the geolocation database is not available, it returns an empty string.
func GetGeoLocation(ipStr string, log logger.Logger) string {
	// Initialize the geolocation database if it's not already loaded
	ip2dbOnce.Do(func() {
		// Look for the IP2Location database in possible locations
		dbPath := findIP2LocationDatabase(log)
		if dbPath == "" {
			ip2dbError = fmt.Errorf("IP2Location database not found")
			return
		}

		var err error
		ip2db, err = ip2location.OpenDB(dbPath)
		if err != nil {
			ip2dbError = err
			log.Error("Failed to open IP2Location database", logger.Error(err))
		} else {
			log.Info("Loaded IP2Location database", logger.String("path", dbPath))
		}
	})

	if ip2db == nil || ip2dbError != nil {
		return ""
	}

	// Parse IP address
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	// Look up the country
	result, err := ip2db.Get_country_short(ipStr)
	if err != nil {
		log.Debug("Failed to get country for IP", logger.String("ip", ipStr), logger.Error(err))
		return ""
	}

	if result.Country_short != "" && result.Country_short != "-" {
		return result.Country_short
	}

	return ""
}

// findIP2LocationDatabase looks for the IP2Location database in common locations
func findIP2LocationDatabase(log logger.Logger) string {
	// Common locations to check for the IP2Location database
	locations := []string{
		"./IP2LOCATION-LITE-DB1.BIN",
		"./configs/IP2LOCATION-LITE-DB1.BIN",
		"/etc/api-gateway/IP2LOCATION-LITE-DB1.BIN",
		"/usr/share/ip2location/IP2LOCATION-LITE-DB1.BIN",
	}

	// First check environment variable
	if envPath := os.Getenv("IP2LOCATION_DB_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
		log.Warn("IP2Location database specified in IP2LOCATION_DB_PATH not found",
			logger.String("path", envPath))
	}

	// Check common locations
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Look in the executable directory
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		dbPath := filepath.Join(exeDir, "IP2LOCATION-LITE-DB1.BIN")
		if _, err := os.Stat(dbPath); err == nil {
			return dbPath
		}
	}

	log.Warn("IP2Location database not found in any common location. Geolocation features will be disabled.")
	return ""
}
