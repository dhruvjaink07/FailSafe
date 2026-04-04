package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// AndroidExecutorClient provides an HTTP interface to a remote Android executor service
type AndroidExecutorClient struct {
	baseURL    string
	httpClient *http.Client
}

// AAPTResponse is the response structure from the remote executor's /aapt endpoint
type AAPTResponse struct {
	Package  string `json:"package"`
	Activity string `json:"activity"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
}

// NewAndroidExecutorClient creates a new client pointing to the remote executor service
func NewAndroidExecutorClient(executorURL string) *AndroidExecutorClient {
	if executorURL == "" {
		// Default to host.docker.internal for Docker-to-host communication
		executorURL = "http://host.docker.internal:9090"
	}

	// Ensure the URL doesn't have a trailing slash
	executorURL = strings.TrimSuffix(executorURL, "/")

	return &AndroidExecutorClient{
		baseURL: executorURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CallAAPT calls the remote executor to extract APK metadata using aapt
// It returns (package, activity, error)
func (c *AndroidExecutorClient) CallAAPT(apkPath string) (string, string, error) {
	// Encode the APK path as a query parameter
	q := url.QueryEscape(apkPath)
	endpoint := fmt.Sprintf("%s/aapt?apk=%s", c.baseURL, q)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to call remote executor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("remote executor returned status %d: %s", resp.StatusCode, string(body))
	}

	var aaptResp AAPTResponse
	if err := json.NewDecoder(resp.Body).Decode(&aaptResp); err != nil {
		return "", "", fmt.Errorf("failed to decode remote executor response: %w", err)
	}

	if aaptResp.Error != "" {
		return "", "", fmt.Errorf("remote executor error: %s", aaptResp.Error)
	}

	if aaptResp.Package == "" || aaptResp.Activity == "" {
		return "", "", fmt.Errorf("incomplete response from remote executor: package=%q, activity=%q", aaptResp.Package, aaptResp.Activity)
	}

	return aaptResp.Package, aaptResp.Activity, nil
}

// IsAvailable checks if the remote executor service is reachable
func (c *AndroidExecutorClient) IsAvailable() bool {
	healthURL := fmt.Sprintf("%s/health", c.baseURL)
	resp, err := c.httpClient.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// GetExecutorURL returns the configured base URL for the executor service
func (c *AndroidExecutorClient) GetExecutorURL() string {
	return c.baseURL
}

// GetDefaultExecutorURL returns the default executor URL based on environment or standard defaults
func GetDefaultExecutorURL() string {
	// Check for explicit environment variable
	if explicit := strings.TrimSpace(os.Getenv("ANDROID_EXECUTOR_URL")); explicit != "" {
		return explicit
	}

	// Default to host.docker.internal for Docker-to-host communication
	return "http://host.docker.internal:9090"
}
