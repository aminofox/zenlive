package cdn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// CDNProvider represents a CDN provider
type CDNProvider string

const (
	// CDNProviderCloudFlare represents CloudFlare CDN
	CDNProviderCloudFlare CDNProvider = "cloudflare"
	// CDNProviderAWS represents AWS CloudFront
	CDNProviderAWS CDNProvider = "aws"
	// CDNProviderAzure represents Azure CDN
	CDNProviderAzure CDNProvider = "azure"
	// CDNProviderCustom represents a custom CDN
	CDNProviderCustom CDNProvider = "custom"
)

// CDNConfig represents CDN configuration
type CDNConfig struct {
	Provider  CDNProvider       // CDN provider
	BaseURL   string            // CDN base URL
	AccessKey string            // API access key
	SecretKey string            // API secret key
	Zone      string            // CDN zone ID
	CacheTTL  time.Duration     // Default cache TTL
	Headers   map[string]string // Custom headers
	Enabled   bool              // Whether CDN is enabled
}

// CDNClient manages CDN operations
type CDNClient struct {
	config     CDNConfig
	httpClient *http.Client
}

// NewCDNClient creates a new CDN client
func NewCDNClient(config CDNConfig) *CDNClient {
	if config.CacheTTL == 0 {
		config.CacheTTL = 24 * time.Hour
	}

	return &CDNClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UploadFile uploads a file to the CDN
func (c *CDNClient) UploadFile(ctx context.Context, path string, content io.Reader, contentType string) (string, error) {
	if !c.config.Enabled {
		return "", errors.New("CDN is not enabled")
	}

	// Build the full URL
	cdnURL := c.buildURL(path)

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, cdnURL, content)
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("Content-Type", contentType)
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	// Set cache control
	req.Header.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(c.config.CacheTTL.Seconds())))

	// Add authentication
	c.addAuth(req)

	// Send the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("CDN upload failed with status %d", resp.StatusCode)
	}

	return cdnURL, nil
}

// DeleteFile deletes a file from the CDN
func (c *CDNClient) DeleteFile(ctx context.Context, path string) error {
	if !c.config.Enabled {
		return errors.New("CDN is not enabled")
	}

	// Build the full URL
	cdnURL := c.buildURL(path)

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, cdnURL, nil)
	if err != nil {
		return err
	}

	// Add authentication
	c.addAuth(req)

	// Send the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("CDN delete failed with status %d", resp.StatusCode)
	}

	return nil
}

// PurgeCache purges the CDN cache for a file
func (c *CDNClient) PurgeCache(ctx context.Context, paths []string) error {
	if !c.config.Enabled {
		return errors.New("CDN is not enabled")
	}

	switch c.config.Provider {
	case CDNProviderCloudFlare:
		return c.purgeCloudFlareCache(ctx, paths)
	case CDNProviderAWS:
		return c.purgeAWSCache(ctx, paths)
	case CDNProviderAzure:
		return c.purgeAzureCache(ctx, paths)
	default:
		return errors.New("cache purge not supported for this provider")
	}
}

// GetURL returns the CDN URL for a file
func (c *CDNClient) GetURL(path string) string {
	if !c.config.Enabled {
		return path
	}

	return c.buildURL(path)
}

// IsEnabled returns whether the CDN is enabled
func (c *CDNClient) IsEnabled() bool {
	return c.config.Enabled
}

// buildURL builds the full CDN URL
func (c *CDNClient) buildURL(path string) string {
	baseURL := c.config.BaseURL
	if baseURL == "" {
		return path
	}

	// Remove leading slash from path
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// Ensure base URL doesn't end with slash
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	return baseURL + "/" + path
}

// addAuth adds authentication to the request
func (c *CDNClient) addAuth(req *http.Request) {
	switch c.config.Provider {
	case CDNProviderCloudFlare:
		req.Header.Set("X-Auth-Email", c.config.AccessKey)
		req.Header.Set("X-Auth-Key", c.config.SecretKey)
	case CDNProviderAWS:
		// AWS uses signature v4 - simplified here
		req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s", c.config.AccessKey))
	case CDNProviderAzure:
		req.Header.Set("x-ms-blob-type", "BlockBlob")
	}
}

// purgeCloudFlareCache purges CloudFlare cache
func (c *CDNClient) purgeCloudFlareCache(ctx context.Context, paths []string) error {
	// Build CloudFlare API URL
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", c.config.Zone)

	// Build request body
	body := map[string]interface{}{
		"files": paths,
	}

	// Send purge request (simplified - would need actual implementation)
	_ = apiURL
	_ = body

	return nil
}

// purgeAWSCache purges AWS CloudFront cache
func (c *CDNClient) purgeAWSCache(ctx context.Context, paths []string) error {
	// AWS CloudFront invalidation (simplified)
	return nil
}

// purgeAzureCache purges Azure CDN cache
func (c *CDNClient) purgeAzureCache(ctx context.Context, paths []string) error {
	// Azure CDN purge (simplified)
	return nil
}

// EdgeLocation represents a CDN edge location
type EdgeLocation struct {
	ID      string  // Location ID
	City    string  // City name
	Country string  // Country code
	Lat     float64 // Latitude
	Lon     float64 // Longitude
}

// CDNStats represents CDN statistics
type CDNStats struct {
	TotalRequests int64          // Total requests
	CacheHits     int64          // Cache hits
	CacheMisses   int64          // Cache misses
	BytesServed   int64          // Bytes served
	HitRate       float64        // Cache hit rate
	EdgeLocations []EdgeLocation // Active edge locations
}

// GetStats returns CDN statistics
func (c *CDNClient) GetStats(ctx context.Context) (CDNStats, error) {
	if !c.config.Enabled {
		return CDNStats{}, errors.New("CDN is not enabled")
	}

	// This would call the CDN provider's API to get stats
	// Simplified implementation here

	stats := CDNStats{
		TotalRequests: 0,
		CacheHits:     0,
		CacheMisses:   0,
		BytesServed:   0,
	}

	if stats.TotalRequests > 0 {
		stats.HitRate = float64(stats.CacheHits) / float64(stats.TotalRequests)
	}

	return stats, nil
}

// URLSigner signs URLs for secure CDN access
type URLSigner struct {
	secretKey string
	expiry    time.Duration
}

// NewURLSigner creates a new URL signer
func NewURLSigner(secretKey string, expiry time.Duration) *URLSigner {
	if expiry == 0 {
		expiry = 1 * time.Hour
	}

	return &URLSigner{
		secretKey: secretKey,
		expiry:    expiry,
	}
}

// SignURL signs a URL with expiration
func (us *URLSigner) SignURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// Add expiration timestamp
	expires := time.Now().Add(us.expiry).Unix()

	// Add query parameters
	query := parsedURL.Query()
	query.Set("expires", fmt.Sprintf("%d", expires))

	// Generate signature (simplified - would use HMAC in production)
	signature := fmt.Sprintf("%x", []byte(us.secretKey+urlStr))
	query.Set("signature", signature[:16])

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

// VerifyURL verifies a signed URL
func (us *URLSigner) VerifyURL(urlStr string) (bool, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false, err
	}

	query := parsedURL.Query()

	// Check expiration
	expiresStr := query.Get("expires")
	if expiresStr == "" {
		return false, errors.New("missing expiration")
	}

	var expires int64
	fmt.Sscanf(expiresStr, "%d", &expires)

	if time.Now().Unix() > expires {
		return false, errors.New("URL expired")
	}

	// Verify signature (simplified)
	signature := query.Get("signature")
	if signature == "" {
		return false, errors.New("missing signature")
	}

	return true, nil
}

// CDNMiddleware creates middleware for CDN integration
type CDNMiddleware struct {
	client *CDNClient
	signer *URLSigner
}

// NewCDNMiddleware creates a new CDN middleware
func NewCDNMiddleware(client *CDNClient, signer *URLSigner) *CDNMiddleware {
	return &CDNMiddleware{
		client: client,
		signer: signer,
	}
}

// Handler returns an HTTP handler that rewrites URLs to use CDN
func (cm *CDNMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cm.client.IsEnabled() {
			// Rewrite static asset URLs to use CDN
			// This is a simplified example
			r.URL.Path = cm.client.GetURL(r.URL.Path)
		}

		next.ServeHTTP(w, r)
	})
}
