package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"api-gateway/internal/config"
	"api-gateway/internal/util"
	"api-gateway/pkg/logger"

	"github.com/gorilla/websocket"
)

// WSProxy handles WebSocket connections to upstream services
type WSProxy struct {
	config *config.Config
	routes *config.RouteConfig
	log    logger.Logger
	// Websocket upgrader
	upgrader websocket.Upgrader
}

// NewWSProxy creates a new WebSocket proxy
func NewWSProxy(config *config.Config, routes *config.RouteConfig, log logger.Logger) *WSProxy {
	return &WSProxy{
		config: config,
		routes: routes,
		log:    log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// Allow all origins
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ProxyWebSocket handles WebSocket proxy requests
func (p *WSProxy) ProxyWebSocket(route config.Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route.WebSocket == nil || !route.WebSocket.Enabled {
			http.Error(w, "WebSocket not enabled for this route", http.StatusBadRequest)
			return
		}

		// Log WebSocket connection request
		p.log.Debug("Received WebSocket connection request",
			logger.String("path", r.URL.Path),
			logger.String("query", r.URL.RawQuery),
			logger.String("remote_addr", r.RemoteAddr),
			logger.String("x-forwarded-for", r.Header.Get("X-Forwarded-For")),
			logger.String("x-real-ip", r.Header.Get("X-Real-IP")),
		)

		// Parse the upstream URL
		upstreamURL, err := url.Parse(route.Upstream)
		if err != nil {
			p.log.Error("Failed to parse upstream URL",
				logger.String("upstream", route.Upstream),
				logger.Error(err),
			)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Determine the WebSocket path
		wsPath := route.WebSocket.UpstreamPath
		if wsPath == "" {
			// If no specific upstream path is provided, use the same path
			wsPath = r.URL.Path
			if route.StripPrefix && strings.HasPrefix(wsPath, route.Path) {
				// Strip the route prefix if needed
				wsPath = strings.TrimPrefix(wsPath, route.Path)
				if wsPath == "" {
					wsPath = "/"
				}
			}
		}

		// Form the WebSocket URL
		wsURL := url.URL{
			Scheme: convertHttpSchemeToWs(upstreamURL.Scheme),
			Host:   upstreamURL.Host,
			Path:   wsPath,
		}

		// Add query parameters if any
		if r.URL.RawQuery != "" {
			wsURL.RawQuery = r.URL.RawQuery
		}

		p.log.Debug("Upgrading to WebSocket connection",
			logger.String("upstream", wsURL.String()),
		)

		// Upgrade the client connection
		clientConn, err := p.upgrader.Upgrade(w, r, nil)
		if err != nil {
			p.log.Error("Failed to upgrade client connection", logger.Error(err))
			return
		}
		defer clientConn.Close()

		// Copy headers for upstream connection
		headers := http.Header{}
		for k, vs := range r.Header {
			if k == "Connection" || k == "Sec-Websocket-Key" ||
				k == "Sec-Websocket-Version" || k == "Sec-Websocket-Extensions" ||
				k == "Sec-Websocket-Protocol" || k == "Upgrade" {
				continue
			}
			for _, v := range vs {
				headers.Add(k, v)
			}
		}

		headers.Set("Host", upstreamURL.Host)
		headers.Set("Origin", fmt.Sprintf("%s://%s", upstreamURL.Scheme, upstreamURL.Host))

		// Extract the real client IP
		clientIP := util.GetClientIP(r)
		p.log.Debug("Extracted client IP for WebSocket proxy",
			logger.String("remote_addr", r.RemoteAddr),
			logger.String("client_ip", clientIP),
			logger.String("xff_header", r.Header.Get("X-Forwarded-For")),
			logger.String("xrip_header", r.Header.Get("X-Real-IP")),
		)

		// Add custom headers to preserve client IP information
		if clientIP != "" {
			// For X-Forwarded-For, handle existing chains properly
			if xffHeader := r.Header.Get("X-Forwarded-For"); xffHeader != "" {
				// Keep existing chain but only if clientIP is not already there
				// to avoid duplicating the client IP
				if !strings.HasPrefix(xffHeader, clientIP+",") && xffHeader != clientIP {
					headers.Set("X-Forwarded-For", xffHeader)
					p.log.Debug("Preserved X-Forwarded-For header", logger.String("value", xffHeader))
				}
			} else {
				// No existing chain, just set the client IP
				headers.Set("X-Forwarded-For", clientIP)
				p.log.Debug("Set X-Forwarded-For header", logger.String("value", clientIP))
			}

			// Always set X-Real-IP to the original client IP
			headers.Set("X-Real-IP", clientIP)
			p.log.Debug("Set X-Real-IP header", logger.String("value", clientIP))
		}

		// Try to resolve country from IP if possible
		country := util.GetGeoLocation(clientIP, p.log)
		if country != "" {
			headers.Set("X-Client-Geo-Country", country)
			p.log.Debug("Set X-Client-Geo-Country header",
				logger.String("ip", clientIP),
				logger.String("country", country))
		} else {
			p.log.Debug("No country information available for IP",
				logger.String("ip", clientIP))
		}

		headers.Set("X-Forwarded-Host", r.Host)
		headers.Set("X-Gateway-Proxy", "true")

		// Check for token in URL query parameters and add it to the headers if present
		// This ensures backward compatibility with clients that send tokens in URL
		token := r.URL.Query().Get("token")
		if token != "" && headers.Get("Authorization") == "" {
			headers.Set("Authorization", "Bearer "+token)
			p.log.Debug("Added token from URL query to Authorization header")
		}

		// Connect to upstream WebSocket
		p.log.Debug("Connecting to upstream WebSocket",
			logger.String("url", wsURL.String()),
		)
		upstreamConn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), headers)
		if err != nil {
			p.log.Error("Failed to connect to upstream WebSocket", logger.Error(err))
			clientConn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Cannot connect to service"))
			return
		}
		defer upstreamConn.Close()

		p.log.Debug("WebSocket connection established",
			logger.String("path", r.URL.Path),
			logger.String("upstream", wsURL.String()),
			logger.String("client_ip", clientIP),
			logger.String("country", country),
		)

		// Bidirectional copy
		errorChan := make(chan error, 2)

		// Client to upstream
		go p.proxyWebSocketConn(clientConn, upstreamConn, errorChan)

		// Upstream to client
		go p.proxyWebSocketConn(upstreamConn, clientConn, errorChan)

		// Wait for an error in either direction
		err = <-errorChan
		if err != nil && err != io.EOF {
			p.log.Error("WebSocket proxy error", logger.Error(err))
		}

		p.log.Debug("WebSocket connection closed",
			logger.String("path", r.URL.Path),
		)
	})
}

// proxyWebSocketConn copies messages from one connection to another
func (p *WSProxy) proxyWebSocketConn(src, dst *websocket.Conn, errChan chan error) {
	for {
		messageType, message, err := src.ReadMessage()
		if err != nil {
			// Don't log EOF as error - it's normal when connection closes
			if err != io.EOF && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				errChan <- fmt.Errorf("error reading from WebSocket: %w", err)
			} else {
				errChan <- io.EOF
			}
			break
		}

		if err := dst.WriteMessage(messageType, message); err != nil {
			errChan <- fmt.Errorf("error writing to WebSocket: %w", err)
			break
		}
	}
}

// convertHttpSchemeToWs converts HTTP/HTTPS scheme to WS/WSS
func convertHttpSchemeToWs(scheme string) string {
	scheme = strings.ToLower(scheme)
	if scheme == "https" {
		return "wss"
	}
	return "ws"
}
