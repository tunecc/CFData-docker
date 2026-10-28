package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultR2Filename = "cfdata-targets.txt"
	r2UploadAttempts  = 3
	r2RequestTimeout  = 30 * time.Second
)

var (
	r2RetryDelay    = 2 * time.Second
	r2ClientFactory = func() *http.Client {
		return &http.Client{Transport: managementTransport(), Timeout: r2RequestTimeout}
	}
	r2Logger = func(format string, args ...interface{}) {
		fmt.Printf(format+"\n", args...)
	}
)

type r2AuthError struct {
	status int
	body   string
}

func (e *r2AuthError) Error() string {
	message := fmt.Sprintf("R2 鉴权失败（HTTP %d），请检查 worker-token", e.status)
	if e.body != "" {
		message += ": " + e.body
	}
	return message
}

func uploadR2Content(workerURL, workerToken, filename, content string) error {
	if strings.TrimSpace(content) == "" {
		r2Logger("[r2] 导出内容为空，跳过上传")
		return nil
	}
	endpoint, parsed, err := buildR2Endpoint(workerURL, workerToken)
	if err != nil {
		return err
	}
	if filename = strings.TrimSpace(filename); filename == "" {
		filename = defaultR2Filename
	}
	payload, err := json.Marshal(map[string]string{"filename": filename, "value": content})
	if err != nil {
		return fmt.Errorf("R2 上传内容编码失败: %w", err)
	}

	client := r2ClientFactory()
	var lastErr error
	for attempt := 1; attempt <= r2UploadAttempts; attempt++ {
		lastErr = doR2Request(client, endpoint, payload)
		if lastErr == nil {
			r2Logger("[r2] 上传成功: %s (%d 字节)", r2SafeEndpoint(parsed), len(content))
			return nil
		}
		var authErr *r2AuthError
		if errors.As(lastErr, &authErr) {
			return lastErr
		}
		r2Logger("[r2] 第 %d/%d 次上传失败: %v", attempt, r2UploadAttempts, lastErr)
		if attempt < r2UploadAttempts {
			time.Sleep(r2RetryDelay)
		}
	}
	return fmt.Errorf("R2 上传失败（已尝试 %d 次）: %w", r2UploadAttempts, lastErr)
}

func buildR2Endpoint(rawURL, token string) (string, *url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil, errors.New("R2 上传已启用，但 worker-url 未配置")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", nil, errors.New("R2 上传已启用，但 worker-token 未配置")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", nil, fmt.Errorf("R2 worker-url 必须是有效的 http/https 地址")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/storage"
	query := parsed.Query()
	query.Set("token", token)
	parsed.RawQuery = query.Encode()
	return parsed.String(), parsed, nil
}

func r2SafeEndpoint(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	safe := *parsed
	safe.RawQuery = ""
	safe.Fragment = ""
	return safe.String()
}

func doR2Request(client *http.Client, endpoint string, payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("R2 请求构建失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("R2 请求失败: %s", r2UnderlyingError(err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	message := truncateR2Body(redactR2Secrets(strings.TrimSpace(string(body)), endpoint))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &r2AuthError{status: resp.StatusCode, body: message}
	}
	if message == "" {
		return fmt.Errorf("R2 返回 HTTP %d", resp.StatusCode)
	}
	return fmt.Errorf("R2 返回 HTTP %d: %s", resp.StatusCode, message)
}

func r2UnderlyingError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err.Error()
	}
	return err.Error()
}

var r2TokenPattern = regexp.MustCompile(`(?i)(token=)[^&\s"']+`)

// Worker 或代理可能在响应体回显请求 URL，日志和错误中不得出现 token
func redactR2Secrets(text, endpoint string) string {
	if text == "" {
		return text
	}
	if parsed, err := url.Parse(endpoint); err == nil {
		if token := parsed.Query().Get("token"); token != "" {
			text = strings.ReplaceAll(text, token, "***")
			text = strings.ReplaceAll(text, url.QueryEscape(token), "***")
		}
	}
	return r2TokenPattern.ReplaceAllString(text, "${1}***")
}

func truncateR2Body(body string) string {
	if len(body) <= 200 {
		return body
	}
	return body[:200] + "..."
}
