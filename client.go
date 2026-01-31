package processorsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiPathPrefix  = "/api/v1"
	defaultTimeout = 10 * time.Second
)

// Config holds configuration for the processor service client
type Config struct {
	BaseURL string        // Processor service base URL (e.g. "http://localhost:3003")
	Timeout time.Duration // Request timeout (default 10 seconds)
}

// Client is the processor service HTTP client
type Client struct {
	baseURL string
	client  *http.Client
}

// APIError represents an error returned by the processor service API
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("processor service returned status %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("processor service returned status %d: %s", e.StatusCode, e.Body)
}

// IsAPIError checks if an error is an APIError and returns it
func IsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if err != nil && errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

func parseErrorResponse(statusCode int, body []byte) *APIError {
	var errorResp struct {
		Message string `json:"message"`
		Success bool   `json:"success"`
		Status  int    `json:"status"`
		Error   string `json:"error"`
	}
	bodyStr := string(body)
	if err := json.Unmarshal(body, &errorResp); err == nil && (errorResp.Message != "" || errorResp.Error != "") {
		errMessage := errorResp.Error
		if errMessage == "" {
			errMessage = errorResp.Message
		}
		return &APIError{StatusCode: statusCode, Message: errMessage, Body: bodyStr}
	}
	return &APIError{StatusCode: statusCode, Message: bodyStr, Body: bodyStr}
}

func statusIn(code int, codes []int) bool {
	for _, c := range codes {
		if code == c {
			return true
		}
	}
	return false
}

func pathSeg(s string) string { return url.PathEscape(s) }

func (c *Client) do(ctx context.Context, method, path string, body []byte, successStatuses []int, result interface{}, wrapErr string) error {
	var req *http.Request
	var err error
	if len(body) > 0 {
		req, err = http.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, path, nil)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", wrapErr, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", wrapErr, err)
	}
	defer resp.Body.Close()

	if !statusIn(resp.StatusCode, successStatuses) {
		respBody, _ := io.ReadAll(resp.Body)
		return parseErrorResponse(resp.StatusCode, respBody)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("%s: %w", wrapErr, err)
		}
	}
	return nil
}

// NewClient creates a new processor service client
func NewClient(config Config) (*Client, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

// Pagination contains pagination metadata (matches processor-service / go-pkg-utils)
type Pagination struct {
	Page         int   `json:"page"`
	PerPage      int   `json:"perPage"`
	Total        int64 `json:"total"`
	TotalPages   int   `json:"totalPages"`
	HasNext      bool  `json:"hasNext"`
	HasPrevious  bool  `json:"hasPrevious"`
	NextPage     *int  `json:"nextPage,omitempty"`
	PreviousPage *int  `json:"previousPage,omitempty"`
}

// EventItem represents an event in list/detail responses
type EventItem struct {
	ID        string                 `json:"id"`
	Service   string                 `json:"service"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt string                 `json:"createdAt"`
	UpdatedAt string                 `json:"updatedAt"`
}

// ListEventsResponse is the paginated response from listing events
type ListEventsResponse struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message"`
	Status     int         `json:"status"`
	Data       []EventItem `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// GetEventResponse is the response from getting an event
type GetEventResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	Status  int       `json:"status"`
	Data    EventItem `json:"data"`
}

// ListEvents lists events by forwarding the raw query string to processor-service
func (c *Client) ListEvents(ctx context.Context, queryString string) (*ListEventsResponse, error) {
	path := c.baseURL + apiPathPrefix + "/events"
	if queryString != "" {
		path += "?" + queryString
	}
	var result ListEventsResponse
	err := c.do(ctx, http.MethodGet, path, nil, []int{http.StatusOK}, &result, "failed to list events")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEvent gets an event by ID
func (c *Client) GetEvent(ctx context.Context, id string) (*GetEventResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("event id is required")
	}
	path := c.baseURL + apiPathPrefix + "/events/" + pathSeg(id)
	var result GetEventResponse
	err := c.do(ctx, http.MethodGet, path, nil, []int{http.StatusOK}, &result, "failed to get event")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateEvent updates an event by ID (payload only)
func (c *Client) UpdateEvent(ctx context.Context, id string, payload map[string]interface{}) (*GetEventResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("event id is required")
	}
	path := c.baseURL + apiPathPrefix + "/events/" + pathSeg(id)
	body, _ := json.Marshal(map[string]interface{}{"payload": payload})
	var result GetEventResponse
	err := c.do(ctx, http.MethodPut, path, body, []int{http.StatusOK}, &result, "failed to update event")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteEvent deletes an event by ID
func (c *Client) DeleteEvent(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("event id is required")
	}
	path := c.baseURL + apiPathPrefix + "/events/" + pathSeg(id)
	return c.do(ctx, http.MethodDelete, path, nil, []int{http.StatusOK}, nil, "failed to delete event")
}

// ScriptItem represents a script in list/detail responses
type ScriptItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Service   string `json:"service"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Code      string `json:"code"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ListScriptsResponse is the paginated response from listing scripts
type ListScriptsResponse struct {
	Success    bool         `json:"success"`
	Message    string       `json:"message"`
	Status     int          `json:"status"`
	Data       []ScriptItem `json:"data"`
	Pagination *Pagination  `json:"pagination,omitempty"`
}

// GetScriptResponse is the response from getting a script
type GetScriptResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Status  int        `json:"status"`
	Data    ScriptItem `json:"data"`
}

// CreateScriptBody is the body for creating a script
type CreateScriptBody struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	Type    string `json:"type"`
	Version string `json:"version,omitempty"`
	Code    string `json:"code"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// UpdateScriptBody is the body for updating a script
type UpdateScriptBody struct {
	Name    *string `json:"name,omitempty"`
	Service *string `json:"service,omitempty"`
	Type    *string `json:"type,omitempty"`
	Version *string `json:"version,omitempty"`
	Code    *string `json:"code,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

// ListScripts lists scripts by forwarding the raw query string
func (c *Client) ListScripts(ctx context.Context, queryString string) (*ListScriptsResponse, error) {
	path := c.baseURL + apiPathPrefix + "/scripts"
	if queryString != "" {
		path += "?" + queryString
	}
	var result ListScriptsResponse
	err := c.do(ctx, http.MethodGet, path, nil, []int{http.StatusOK}, &result, "failed to list scripts")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetScript gets a script by ID
func (c *Client) GetScript(ctx context.Context, id string) (*GetScriptResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("script id is required")
	}
	path := c.baseURL + apiPathPrefix + "/scripts/" + pathSeg(id)
	var result GetScriptResponse
	err := c.do(ctx, http.MethodGet, path, nil, []int{http.StatusOK}, &result, "failed to get script")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateScript creates a script
func (c *Client) CreateScript(ctx context.Context, body CreateScriptBody) (*GetScriptResponse, error) {
	path := c.baseURL + apiPathPrefix + "/scripts"
	raw, _ := json.Marshal(body)
	var result GetScriptResponse
	err := c.do(ctx, http.MethodPost, path, raw, []int{http.StatusCreated}, &result, "failed to create script")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateScript updates a script by ID
func (c *Client) UpdateScript(ctx context.Context, id string, body UpdateScriptBody) (*GetScriptResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("script id is required")
	}
	path := c.baseURL + apiPathPrefix + "/scripts/" + pathSeg(id)
	raw, _ := json.Marshal(body)
	var result GetScriptResponse
	err := c.do(ctx, http.MethodPut, path, raw, []int{http.StatusOK}, &result, "failed to update script")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteScript deletes a script by ID
func (c *Client) DeleteScript(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("script id is required")
	}
	path := c.baseURL + apiPathPrefix + "/scripts/" + pathSeg(id)
	return c.do(ctx, http.MethodDelete, path, nil, []int{http.StatusOK}, nil, "failed to delete script")
}

// ListScriptExecutionsByScriptID lists script executions for a script
func (c *Client) ListScriptExecutionsByScriptID(ctx context.Context, scriptID string, queryString string) (*ListScriptExecutionsResponse, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("script id is required")
	}
	path := c.baseURL + apiPathPrefix + "/scripts/" + pathSeg(scriptID) + "/executions"
	if queryString != "" {
		path += "?" + queryString
	}
	var result ListScriptExecutionsResponse
	err := c.do(ctx, http.MethodGet, path, nil, []int{http.StatusOK}, &result, "failed to list script executions")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ScriptExecutionItem represents a script execution in list/detail responses
type ScriptExecutionItem struct {
	ID            string  `json:"id"`
	ScriptID      string  `json:"scriptId"`
	EventID       string  `json:"eventId"`
	ScriptVersion string  `json:"scriptVersion"`
	ScriptName    string  `json:"scriptName"`
	Status        string  `json:"status"`
	ErrorMessage  *string `json:"errorMessage,omitempty"`
	DurationMs    int64   `json:"durationMs"`
	ExecutedAt    string  `json:"executedAt"`
}

// ListScriptExecutionsResponse is the paginated response from listing script executions
type ListScriptExecutionsResponse struct {
	Success    bool                  `json:"success"`
	Message    string                `json:"message"`
	Status     int                   `json:"status"`
	Data       []ScriptExecutionItem `json:"data"`
	Pagination *Pagination           `json:"pagination,omitempty"`
}

// GetScriptExecutionResponse is the response from getting a script execution
type GetScriptExecutionResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Status  int                 `json:"status"`
	Data    ScriptExecutionItem `json:"data"`
}

// ListScriptExecutions lists script executions by forwarding the raw query string
func (c *Client) ListScriptExecutions(ctx context.Context, queryString string) (*ListScriptExecutionsResponse, error) {
	path := c.baseURL + apiPathPrefix + "/script-executions"
	if queryString != "" {
		path += "?" + queryString
	}
	var result ListScriptExecutionsResponse
	err := c.do(ctx, http.MethodGet, path, nil, []int{http.StatusOK}, &result, "failed to list script executions")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetScriptExecution gets a script execution by ID
func (c *Client) GetScriptExecution(ctx context.Context, id string) (*GetScriptExecutionResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("script execution id is required")
	}
	path := c.baseURL + apiPathPrefix + "/script-executions/" + pathSeg(id)
	var result GetScriptExecutionResponse
	err := c.do(ctx, http.MethodGet, path, nil, []int{http.StatusOK}, &result, "failed to get script execution")
	if err != nil {
		return nil, err
	}
	return &result, nil
}
