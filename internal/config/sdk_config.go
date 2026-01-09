// Package config provides configuration management for the CLI Proxy API server.
// It handles loading and parsing YAML configuration files, and provides structured
// access to application settings including server port, authentication directory,
// debug settings, proxy configuration, and API keys.
package config

import (
	"sync"
	"sync/atomic"
)

// ApiKeyEntry represents an API key with extended metadata for management.
type ApiKeyEntry struct {
	// ID is a stable unique identifier for this key entry (UUID format).
	ID string `yaml:"id,omitempty" json:"id,omitempty"`

	// Key is the actual API key value.
	Key string `yaml:"api-key" json:"api-key"`

	// Name is an optional human-readable name for the key.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// IsActive indicates whether the key is currently enabled.
	IsActive bool `yaml:"is-active" json:"is-active"`

	// UsageCount tracks how many times this key has been used.
	// Use atomic operations for thread-safe updates.
	UsageCount int64 `yaml:"usage-count,omitempty" json:"usage-count,omitempty"`

	// LastUsedAt is the ISO 8601 timestamp of the last usage.
	LastUsedAt string `yaml:"last-used-at,omitempty" json:"last-used-at,omitempty"`

	// CreatedAt is the ISO 8601 timestamp when this key was created.
	CreatedAt string `yaml:"created-at,omitempty" json:"created-at,omitempty"`

	// mu protects LastUsedAt updates
	mu sync.Mutex `yaml:"-" json:"-"`
}

// IncrementUsage atomically increments the usage count and updates last used time.
func (e *ApiKeyEntry) IncrementUsage(timestamp string) {
	atomic.AddInt64(&e.UsageCount, 1)
	e.mu.Lock()
	e.LastUsedAt = timestamp
	e.mu.Unlock()
}

// GetUsageCount returns the current usage count atomically.
func (e *ApiKeyEntry) GetUsageCount() int64 {
	return atomic.LoadInt64(&e.UsageCount)
}

// SDKConfig represents the application's configuration, loaded from a YAML file.
type SDKConfig struct {
	// ProxyURL is the URL of an optional proxy server to use for outbound requests.
	ProxyURL string `yaml:"proxy-url" json:"proxy-url"`

	// ForceModelPrefix requires explicit model prefixes (e.g., "teamA/gemini-3-pro-preview")
	// to target prefixed credentials. When false, unprefixed model requests may use prefixed
	// credentials as well.
	ForceModelPrefix bool `yaml:"force-model-prefix" json:"force-model-prefix"`

	// RequestLog enables or disables detailed request logging functionality.
	RequestLog bool `yaml:"request-log" json:"request-log"`

	// APIKeys is a list of keys for authenticating clients to this proxy server.
	// Supports both simple string format (for backward compatibility) and extended ApiKeyEntry format.
	APIKeys []ApiKeyEntry `yaml:"api-keys" json:"api-keys"`

	// Access holds request authentication provider configuration.
	Access AccessConfig `yaml:"auth,omitempty" json:"auth,omitempty"`

	// Streaming configures server-side streaming behavior (keep-alives and safe bootstrap retries).
	Streaming StreamingConfig `yaml:"streaming" json:"streaming"`
}

// StreamingConfig holds server streaming behavior configuration.
type StreamingConfig struct {
	// KeepAliveSeconds controls how often the server emits SSE heartbeats (": keep-alive\n\n").
	// <= 0 disables keep-alives. Default is 0.
	KeepAliveSeconds int `yaml:"keepalive-seconds,omitempty" json:"keepalive-seconds,omitempty"`

	// BootstrapRetries controls how many times the server may retry a streaming request before any bytes are sent,
	// to allow auth rotation / transient recovery.
	// <= 0 disables bootstrap retries. Default is 0.
	BootstrapRetries int `yaml:"bootstrap-retries,omitempty" json:"bootstrap-retries,omitempty"`
}

// AccessConfig groups request authentication providers.
type AccessConfig struct {
	// Providers lists configured authentication providers.
	Providers []AccessProvider `yaml:"providers,omitempty" json:"providers,omitempty"`
}

// AccessProvider describes a request authentication provider entry.
type AccessProvider struct {
	// Name is the instance identifier for the provider.
	Name string `yaml:"name" json:"name"`

	// Type selects the provider implementation registered via the SDK.
	Type string `yaml:"type" json:"type"`

	// SDK optionally names a third-party SDK module providing this provider.
	SDK string `yaml:"sdk,omitempty" json:"sdk,omitempty"`

	// APIKeys lists inline keys for providers that require them.
	APIKeys []string `yaml:"api-keys,omitempty" json:"api-keys,omitempty"`

	// Config passes provider-specific options to the implementation.
	Config map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

const (
	// AccessProviderTypeConfigAPIKey is the built-in provider validating inline API keys.
	AccessProviderTypeConfigAPIKey = "config-api-key"

	// DefaultAccessProviderName is applied when no provider name is supplied.
	DefaultAccessProviderName = "config-inline"
)

// ConfigAPIKeyProvider returns the first inline API key provider if present.
func (c *SDKConfig) ConfigAPIKeyProvider() *AccessProvider {
	if c == nil {
		return nil
	}
	for i := range c.Access.Providers {
		if c.Access.Providers[i].Type == AccessProviderTypeConfigAPIKey {
			if c.Access.Providers[i].Name == "" {
				c.Access.Providers[i].Name = DefaultAccessProviderName
			}
			return &c.Access.Providers[i]
		}
	}
	return nil
}

// MakeInlineAPIKeyProvider constructs an inline API key provider configuration.
// It returns nil when no keys are supplied.
func MakeInlineAPIKeyProvider(keys []ApiKeyEntry) *AccessProvider {
	if len(keys) == 0 {
		return nil
	}
	// Extract only active keys
	activeKeys := make([]string, 0, len(keys))
	for _, entry := range keys {
		if entry.IsActive && entry.Key != "" {
			activeKeys = append(activeKeys, entry.Key)
		}
	}
	if len(activeKeys) == 0 {
		return nil
	}
	provider := &AccessProvider{
		Name:    DefaultAccessProviderName,
		Type:    AccessProviderTypeConfigAPIKey,
		APIKeys: activeKeys,
	}
	return provider
}

// ActiveAPIKeyStrings returns a slice of active API key strings from the APIKeys entries.
// This is useful for backward compatibility with code expecting []string.
func (c *SDKConfig) ActiveAPIKeyStrings() []string {
	if c == nil || len(c.APIKeys) == 0 {
		return nil
	}
	result := make([]string, 0, len(c.APIKeys))
	for _, entry := range c.APIKeys {
		if entry.IsActive && entry.Key != "" {
			result = append(result, entry.Key)
		}
	}
	return result
}

// AllAPIKeyStrings returns a slice of all API key strings from the APIKeys entries.
func (c *SDKConfig) AllAPIKeyStrings() []string {
	if c == nil || len(c.APIKeys) == 0 {
		return nil
	}
	result := make([]string, 0, len(c.APIKeys))
	for _, entry := range c.APIKeys {
		if entry.Key != "" {
			result = append(result, entry.Key)
		}
	}
	return result
}
