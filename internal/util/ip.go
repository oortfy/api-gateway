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
	ip2dbEnabled bool
)

// GetClientIP properly extracts the real client IP from the request,
// handling common proxy and forwarding headers.
func GetClientIP(r *http.Request) string {
	// For Nginx, check common headers in order of priority
	// X-Real-IP is most commonly set by Nginx proxy_set_header X-Real-IP $remote_addr
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" && xrip != "unknown" {
		return strings.TrimSpace(xrip)
	}

	// X-Forwarded-For may contain multiple IPs when passing through multiple proxies
	// Format: client, proxy1, proxy2, ...
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		// Get the leftmost (client) IP
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			// Verify it's not internal/unknown
			if clientIP != "" && clientIP != "unknown" {
				return clientIP
			}
		}
	}

	// Check other common headers

	// Cloudflare
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return strings.TrimSpace(cfIP)
	}

	// Akamai and others
	if tcIP := r.Header.Get("True-Client-IP"); tcIP != "" {
		return strings.TrimSpace(tcIP)
	}

	// RFC 7239
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

	// Finally, extract IP from RemoteAddr as last resort
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
		log.Info("Initializing IP2Location database...")

		// Look for the IP2Location database in possible locations
		dbPath := findIP2LocationDatabase(log)
		if dbPath == "" {
			ip2dbError = fmt.Errorf("IP2Location database not found")
			log.Warn("IP2Location database not found. Geolocation features will be disabled.")
			ip2dbEnabled = false
			return
		}

		log.Info("Found IP2Location database", logger.String("path", dbPath))

		// Check if the file is readable
		file, err := os.Open(dbPath)
		if err != nil {
			ip2dbError = fmt.Errorf("cannot open IP2Location database: %w", err)
			log.Warn("Cannot open IP2Location database file. Geolocation features will be disabled.",
				logger.String("path", dbPath),
				logger.Error(err))
			ip2dbEnabled = false
			return
		}
		file.Close()

		// Try to open the database
		ip2db, err = ip2location.OpenDB(dbPath)
		if err != nil {
			ip2dbError = err
			log.Warn("Failed to open IP2Location database. Geolocation features will be disabled.",
				logger.String("path", dbPath),
				logger.Error(err))
			ip2dbEnabled = false
		} else {
			log.Info("Successfully loaded IP2Location database",
				logger.String("path", dbPath))
			ip2dbEnabled = true
		}
	})

	// If IP2Location is not enabled, return empty string without error
	if !ip2dbEnabled {
		return ""
	}

	// Parse IP address
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Debug("Invalid IP address", logger.String("ip", ipStr))
		return ""
	}

	// Look up the country
	result, err := ip2db.Get_country_short(ipStr)
	if err != nil {
		log.Debug("Failed to get country for IP",
			logger.String("ip", ipStr),
			logger.Error(err))
		return ""
	}

	log.Debug("IP2Location lookup result",
		logger.String("ip", ipStr),
		logger.String("country", result.Country_short))

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
		log.Debug("Checking IP2LOCATION_DB_PATH environment variable",
			logger.String("path", envPath))
		if _, err := os.Stat(envPath); err == nil {
			log.Info("Using IP2Location database from environment variable",
				logger.String("path", envPath))
			return envPath
		}
		log.Warn("IP2Location database specified in IP2LOCATION_DB_PATH not found",
			logger.String("path", envPath))
	}

	// Check common locations
	for _, loc := range locations {
		log.Debug("Checking for IP2Location database", logger.String("path", loc))
		if _, err := os.Stat(loc); err == nil {
			log.Info("Found IP2Location database", logger.String("path", loc))
			return loc
		}
	}

	// Get the current working directory for better debugging
	cwd, err := os.Getwd()
	if err == nil {
		log.Debug("Current working directory", logger.String("cwd", cwd))
	}

	// Look in the executable directory
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		log.Debug("Executable directory", logger.String("dir", exeDir))
		dbPath := filepath.Join(exeDir, "IP2LOCATION-LITE-DB1.BIN")
		log.Debug("Checking for IP2Location database in executable dir",
			logger.String("path", dbPath))
		if _, err := os.Stat(dbPath); err == nil {
			log.Info("Found IP2Location database in executable directory",
				logger.String("path", dbPath))
			return dbPath
		}
	} else {
		log.Debug("Could not determine executable path", logger.Error(err))
	}

	log.Warn("IP2Location database not found in any location. Geolocation features will be disabled.")
	return ""
}
