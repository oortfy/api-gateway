package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"api-gateway/internal/config"
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

		// Add custom headers
		headers.Set("X-Forwarded-For", r.RemoteAddr)
		headers.Set("X-Forwarded-Host", r.Host)
		headers.Set("X-Gateway-Proxy", "true")

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
