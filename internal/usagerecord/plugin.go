package usagerecord

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	log "github.com/sirupsen/logrus"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

// Plugin implements coreusage.Plugin to persist usage records to SQLite.
// It captures request/response details from the gin context and stores them
// in the database for later analysis.
type Plugin struct {
	store   *Store
	enabled atomic.Bool
}

var (
	defaultPlugin     *Plugin
	defaultPluginOnce sync.Once
)

// DefaultPlugin returns the global plugin instance.
func DefaultPlugin() *Plugin {
	defaultPluginOnce.Do(func() {
		defaultPlugin = &Plugin{}
		defaultPlugin.enabled.Store(true)
	})
	return defaultPlugin
}

// NewPlugin creates a new usage record plugin.
func NewPlugin(store *Store) *Plugin {
	p := &Plugin{store: store}
	p.enabled.Store(true)
	return p
}

// SetStore sets the store for the default plugin.
func SetStore(store *Store) {
	DefaultPlugin().store = store
}

// SetEnabled enables or disables the plugin.
func (p *Plugin) SetEnabled(enabled bool) {
	if p == nil {
		return
	}
	p.enabled.Store(enabled)
}

// Enabled returns whether the plugin is enabled.
func (p *Plugin) Enabled() bool {
	if p == nil {
		return false
	}
	return p.enabled.Load()
}

// HandleUsage implements coreusage.Plugin.
// It creates a usage record from the provided data and stores it in the database.
func (p *Plugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if p == nil || p.store == nil {
		return
	}
	if !p.enabled.Load() {
		return
	}

	// Extract additional info from gin context if available
	var (
		requestURL      string
		requestMethod   string
		requestHeaders  = make(map[string]string)
		responseHeaders = make(map[string]string)
		statusCode      int
		requestBody     string
		responseBody    string
		isStreaming     bool
		durationMs      int64
		requestID       string
		ip              string
	)

	if ctx != nil {
		if ginCtx, ok := ctx.Value("gin").(*gin.Context); ok && ginCtx != nil {
			ip = ginCtx.ClientIP()
			if ginCtx.Request != nil {
				requestURL = ginCtx.Request.URL.String()
				requestMethod = ginCtx.Request.Method

				// Copy headers (mask sensitive ones)
				for key, values := range ginCtx.Request.Header {
					if len(values) > 0 {
						value := values[0]
						if isSensitiveHeader(key) {
							value = maskValue(value)
						}
						requestHeaders[key] = value
					}
				}
			}

			statusCode = ginCtx.Writer.Status()

			// Capture response headers (mask sensitive ones)
			for key, values := range ginCtx.Writer.Header() {
				if len(values) > 0 {
					value := values[0]
					if isSensitiveHeader(key) {
						value = maskValue(value)
					}
					responseHeaders[key] = value
				}
			}

			// Get request ID using standard utility
			requestID = logging.GetGinRequestID(ginCtx)

			// Check if streaming from context
			if streaming, exists := ginCtx.Get("is_streaming"); exists {
				if streamBool, ok := streaming.(bool); ok {
					isStreaming = streamBool
				}
			}

			// Get duration from context if available, otherwise calculate from start time
			if startTime, exists := ginCtx.Get("request_start_time"); exists {
				if st, ok := startTime.(time.Time); ok {
					durationMs = time.Since(st).Milliseconds()
				}
			}
			if durationMs == 0 && !record.RequestedAt.IsZero() {
				durationMs = time.Since(record.RequestedAt).Milliseconds()
			}

			// Get cached request/response bodies if available
			if body, exists := ginCtx.Get("request_body_for_log"); exists {
				if bodyBytes, ok := body.([]byte); ok {
					requestBody = truncateBody(string(bodyBytes), 50000)
				}
			}
			if body, exists := ginCtx.Get("response_body_for_log"); exists {
				if bodyBytes, ok := body.([]byte); ok {
					responseBody = truncateBody(string(bodyBytes), 50000)
				}
			}
		}
	}

	// Fallback for timestamp
	timestamp := record.RequestedAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// Determine success
	success := !record.Failed
	if statusCode >= 400 {
		success = false
	}

	// Create the record
	rec := &Record{
		RequestID:       requestID,
		Timestamp:       timestamp,
		IP:              ip,
		APIKey:          record.APIKey,
		APIKeyMasked:    MaskAPIKey(record.APIKey),
		Model:           record.Model,
		Provider:        record.Provider,
		IsStreaming:     isStreaming,
		InputTokens:     record.Detail.InputTokens,
		OutputTokens:    record.Detail.OutputTokens,
		TotalTokens:     record.Detail.TotalTokens,
		DurationMs:      durationMs,
		StatusCode:      statusCode,
		Success:         success,
		RequestURL:      requestURL,
		RequestMethod:   requestMethod,
		RequestHeaders:  requestHeaders,
		RequestBody:     requestBody,
		ResponseHeaders: responseHeaders,
		ResponseBody:    responseBody,
	}

	// Calculate total tokens if not provided
	if rec.TotalTokens == 0 {
		rec.TotalTokens = rec.InputTokens + rec.OutputTokens
	}

	// Insert asynchronously to avoid blocking the request
	go func() {
		insertCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := p.store.Insert(insertCtx, rec); err != nil {
			log.WithError(err).Warn("failed to insert usage record")
		}
	}()
}

// isSensitiveHeader returns true for headers that should be masked.
func isSensitiveHeader(key string) bool {
	lower := strings.ToLower(key)
	sensitivePatterns := []string{
		"authorization",
		"x-api-key",
		"api-key",
		"x-goog-api-key",
		"cookie",
		"set-cookie",
		"x-management-key",
	}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// maskValue masks a sensitive value, showing only first and last few characters.
func maskValue(value string) string {
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + "..." + value[len(value)-4:]
}

// truncateBody truncates body content to a maximum length.
func truncateBody(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "\n...[truncated]"
}

// Register registers the default plugin with the core usage manager.
func Register() {
	coreusage.RegisterPlugin(DefaultPlugin())
}
