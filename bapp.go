// Package bapp provides a client for the BAPP Auto API.
package bapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// File represents a file upload.
type File struct {
	Name   string    // filename
	Reader io.Reader // file content
}

// PagedList holds the results slice plus pagination metadata.
type PagedList struct {
	Results  []map[string]interface{} `json:"results"`
	Count    int                      `json:"count"`
	Next     string                   `json:"next"`
	Previous string                   `json:"previous"`
}

// Client is a BAPP Auto API client.
type Client struct {
	Host       string
	Tenant     string
	App        string
	authHeader string
	userAgent  string
	http       *http.Client
}

// Option configures the client.
type Option func(*Client)

// WithBearer sets Bearer token authentication.
func WithBearer(token string) Option {
	return func(c *Client) { c.authHeader = "Bearer " + token }
}

// WithToken sets Token-based authentication.
func WithToken(token string) Option {
	return func(c *Client) { c.authHeader = "Token " + token }
}

// WithTenant sets the default tenant ID.
func WithTenant(id string) Option {
	return func(c *Client) { c.Tenant = id }
}

// WithApp sets the default app slug.
func WithApp(slug string) Option {
	return func(c *Client) { c.App = slug }
}

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// NewClient creates a new BAPP API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		Host: "https://panel.bapp.ro/api",
		App:  "account",
		http: &http.Client{},
	}
	for _, o := range opts {
		o(c)
	}
	c.Host = strings.TrimRight(c.Host, "/")
	return c
}

// hasFiles checks if data contains File values.
func hasFiles(data interface{}) bool {
	m, ok := data.(map[string]interface{})
	if !ok {
		return false
	}
	for _, v := range m {
		if _, isFile := v.(File); isFile {
			return true
		}
	}
	return false
}

func (c *Client) doRaw(method, path string, params url.Values, body interface{}, extraHeaders map[string]string) (json.RawMessage, error) {
	reqURL := c.Host + path
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var r io.Reader
	var contentType string

	if body != nil && hasFiles(body) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for k, v := range body.(map[string]interface{}) {
			switch val := v.(type) {
			case File:
				part, err := w.CreateFormFile(k, filepath.Base(val.Name))
				if err != nil {
					return nil, err
				}
				if _, err := io.Copy(part, val.Reader); err != nil {
					return nil, err
				}
			default:
				_ = w.WriteField(k, fmt.Sprintf("%v", val))
			}
		}
		w.Close()
		r = &buf
		contentType = w.FormDataContentType()
	} else if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		r = bytes.NewReader(b)
		contentType = "application/json"
	}

	req, err := http.NewRequest(method, reqURL, r)
	if err != nil {
		return nil, err
	}

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	if c.Tenant != "" {
		req.Header.Set("x-tenant-id", c.Tenant)
	}
	if c.App != "" {
		req.Header.Set("x-app-slug", c.App)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(rb))
	}
	if resp.StatusCode == 204 {
		return nil, nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return json.RawMessage(raw), nil
}

func (c *Client) do(method, path string, params url.Values, body interface{}, extraHeaders map[string]string) (map[string]interface{}, error) {
	raw, err := c.doRaw(method, path, params, body, extraHeaders)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

// Me returns the current user profile.
func (c *Client) Me() (map[string]interface{}, error) {
	return c.do("GET", "/tasks/bapp_framework.me", nil, nil, map[string]string{"x-app-slug": ""})
}

// GetApp returns app configuration by slug.
func (c *Client) GetApp(appSlug string) (map[string]interface{}, error) {
	return c.do("GET", "/tasks/bapp_framework.getapp", nil, nil, map[string]string{"x-app-slug": appSlug})
}

// ListIntrospect returns list introspect for a content type.
func (c *Client) ListIntrospect(contentType string) (map[string]interface{}, error) {
	return c.do("GET", "/tasks/bapp_framework.listintrospect", url.Values{"ct": {contentType}}, nil, nil)
}

// DetailIntrospect returns detail introspect for a content type.
func (c *Client) DetailIntrospect(contentType string, pk string) (map[string]interface{}, error) {
	p := url.Values{"ct": {contentType}}
	if pk != "" {
		p.Set("pk", pk)
	}
	return c.do("GET", "/tasks/bapp_framework.detailintrospect", p, nil, nil)
}

// List lists entities for a content type. Returns a PagedList with Results, Count, Next, Previous.
func (c *Client) List(contentType string, filters url.Values) (*PagedList, error) {
	raw, err := c.doRaw("GET", "/content-type/"+contentType+"/", filters, nil, nil)
	if err != nil {
		return nil, err
	}
	var page struct {
		Count    int                      `json:"count"`
		Next     string                   `json:"next"`
		Previous string                   `json:"previous"`
		Results  []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decode paged response: %w", err)
	}
	return &PagedList{
		Results:  page.Results,
		Count:    page.Count,
		Next:     page.Next,
		Previous: page.Previous,
	}, nil
}

// Get returns a single entity.
func (c *Client) Get(contentType, id string) (map[string]interface{}, error) {
	return c.do("GET", "/content-type/"+contentType+"/"+id+"/", nil, nil, nil)
}

// Create creates a new entity.
func (c *Client) Create(contentType string, data interface{}) (map[string]interface{}, error) {
	return c.do("POST", "/content-type/"+contentType+"/", nil, data, nil)
}

// Update performs a full update.
func (c *Client) Update(contentType, id string, data interface{}) (map[string]interface{}, error) {
	return c.do("PUT", "/content-type/"+contentType+"/"+id+"/", nil, data, nil)
}

// Patch performs a partial update.
func (c *Client) Patch(contentType, id string, data interface{}) (map[string]interface{}, error) {
	return c.do("PATCH", "/content-type/"+contentType+"/"+id+"/", nil, data, nil)
}

// Delete deletes an entity.
func (c *Client) Delete(contentType, id string) (map[string]interface{}, error) {
	return c.do("DELETE", "/content-type/"+contentType+"/"+id+"/", nil, nil, nil)
}

// DocumentView represents a normalized document view entry extracted from a record.
type DocumentView struct {
	Label            string
	Token            string
	Type             string // "public_view" or "view_token"
	Variations       []map[string]interface{}
	DefaultVariation string
}

// GetDocumentViews extracts available document views from a record.
// Works with both public_view (new) and view_token (legacy) formats.
func GetDocumentViews(record map[string]interface{}) []DocumentView {
	var views []DocumentView

	if pv, ok := record["public_view"].([]interface{}); ok {
		for _, item := range pv {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			dv := DocumentView{
				Type: "public_view",
			}
			if s, ok := entry["label"].(string); ok {
				dv.Label = s
			}
			if s, ok := entry["view_token"].(string); ok {
				dv.Token = s
			}
			if vars, ok := entry["variations"].([]interface{}); ok {
				for _, v := range vars {
					if m, ok := v.(map[string]interface{}); ok {
						dv.Variations = append(dv.Variations, m)
					}
				}
			}
			if s, ok := entry["default_variation"].(string); ok {
				dv.DefaultVariation = s
			}
			views = append(views, dv)
		}
	}

	if vt, ok := record["view_token"].([]interface{}); ok {
		for _, item := range vt {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			dv := DocumentView{
				Type: "view_token",
			}
			if s, ok := entry["label"].(string); ok {
				dv.Label = s
			}
			if s, ok := entry["view_token"].(string); ok {
				dv.Token = s
			}
			views = append(views, dv)
		}
	}

	return views
}

// GetDocumentURL builds a document render/download URL from a record.
// Prefers public_view when both formats are present.
//
//	output: "html", "pdf", "jpg", or "context"
//	label: select a specific view by label (empty string = first available)
//	variation: variation code for public_view entries (empty string = use default)
//	download: when true the response is sent as an attachment
func (c *Client) GetDocumentURL(record map[string]interface{}, output, label, variation string, download bool) string {
	views := GetDocumentViews(record)
	if len(views) == 0 {
		return ""
	}

	var view *DocumentView
	if label != "" {
		for i := range views {
			if views[i].Label == label {
				view = &views[i]
				break
			}
		}
	}
	if view == nil {
		view = &views[0]
	}

	if view.Token == "" {
		return ""
	}

	if view.Type == "public_view" {
		u := fmt.Sprintf("%s/render/%s?output=%s", c.Host, view.Token, output)
		v := variation
		if v == "" {
			v = view.DefaultVariation
		}
		if v != "" {
			u += "&variation=" + v
		}
		if download {
			u += "&download=true"
		}
		return u
	}

	// Legacy view_token
	var action string
	switch output {
	case "pdf":
		if download {
			action = "pdf.download"
		} else {
			action = "pdf.view"
		}
	case "context":
		action = "pdf.context"
	default:
		action = "pdf.preview"
	}
	return fmt.Sprintf("%s/documents/%s?token=%s", c.Host, action, view.Token)
}

// GetDocumentContent fetches document content (PDF, HTML, JPG, etc.) as bytes.
// Builds the URL via GetDocumentURL and performs a plain GET request.
// Returns (nil, nil) when the record has no view tokens.
func (c *Client) GetDocumentContent(record map[string]interface{}, output, label, variation string, download bool) ([]byte, error) {
	u := c.GetDocumentURL(record, output, label, variation, download)
	if u == "" {
		return nil, nil
	}
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// ListTasks returns all available task codes.
func (c *Client) ListTasks() ([]interface{}, error) {
	raw, err := c.doRaw("GET", "/tasks", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	var result []interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}
	return result, nil
}

// DetailTask returns task configuration.
func (c *Client) DetailTask(code string) (map[string]interface{}, error) {
	return c.do("OPTIONS", "/tasks/"+code, nil, nil, nil)
}

// RunTask executes a task. Pass nil payload for GET, non-nil for POST.
func (c *Client) RunTask(code string, payload interface{}) (map[string]interface{}, error) {
	method := "GET"
	if payload != nil {
		method = "POST"
	}
	return c.do(method, "/tasks/"+code, nil, payload, nil)
}

// RunTaskAsync runs a long-running task and polls until finished.
// Returns the final task data which includes "file" when the task produces a download.
func (c *Client) RunTaskAsync(code string, payload interface{}, pollInterval, timeout time.Duration) (map[string]interface{}, error) {
	if pollInterval == 0 {
		pollInterval = time.Second
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	result, err := c.RunTask(code, payload)
	if err != nil {
		return nil, err
	}

	taskID, _ := result["id"].(string)
	if taskID == "" {
		return result, nil
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		page, err := c.List("bapp_framework.taskdata", url.Values{"id": {taskID}})
		if err != nil {
			return nil, err
		}
		if len(page.Results) == 0 {
			continue
		}
		taskData := page.Results[0]
		if failed, _ := taskData["failed"].(bool); failed {
			msg, _ := taskData["message"].(string)
			return nil, fmt.Errorf("task %s failed: %s", code, msg)
		}
		if finished, _ := taskData["finished"].(bool); finished {
			return taskData, nil
		}
	}
	return nil, fmt.Errorf("task %s (%s) did not finish within %v", code, taskID, timeout)
}
