package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

var managementProxyRaw = ""
var managementProxyURL *url.URL

func parseManagementProxy(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if !strings.Contains(value, "://") {
		return nil, fmt.Errorf("代理地址缺少协议头，请使用 socks5://、socks5h://、http:// 或 https:// 开头")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %v", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("代理地址缺少主机")
	}
	if parsed.Port() == "" {
		return nil, fmt.Errorf("代理地址缺少端口")
	}
	switch scheme {
	case "http", "https":
	case "socks5", "socks5h":
		scheme = "socks5"
	default:
		return nil, fmt.Errorf("不支持的代理协议 %s://（支持 socks5://、socks5h://、http://、https://）", parsed.Scheme)
	}
	u := *parsed
	u.Scheme = scheme
	return &u, nil
}

func setManagementProxy(raw string) error {
	parsed, err := parseManagementProxy(raw)
	if err != nil {
		return err
	}
	managementProxyRaw = strings.TrimSpace(raw)
	managementProxyURL = parsed
	managementHTTPClientCache.Lock()
	managementHTTPClientCache.client = nil
	managementHTTPClientCache.Unlock()
	return nil
}

func managementProxyConfigured() bool {
	return managementProxyURL != nil
}

func managementProxyInUse() *url.URL {
	if managementProxyConfigured() {
		return managementProxyURL
	}
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil {
			continue
		}
		switch strings.ToLower(parsed.Scheme) {
		case "socks5", "socks5h", "http", "https":
			return parsed
		}
	}
	return nil
}

// applyProxyURLToTransport 把给定代理 URL 应用到 transport：socks5 走专用拨号器，其余走 HTTP(S) Proxy。
// proxyURL 为 nil 时保持 transport 原状（由调用方决定是否直连）。
func applyProxyURLToTransport(transport *http.Transport, proxyURL *url.URL) *http.Transport {
	if transport == nil || proxyURL == nil {
		return transport
	}
	if proxyURL.Scheme == "socks5" {
		var auth *proxy.Auth
		if proxyURL.User != nil {
			pw, _ := proxyURL.User.Password()
			auth = &proxy.Auth{User: proxyURL.User.Username(), Password: pw}
		}
		dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
		if err != nil {
			fmt.Printf("[proxy] socks5 拨号器构建失败，该请求保持直连: %v\n", err)
			return transport
		}
		transport.Proxy = nil
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return cd.DialContext(ctx, network, addr)
			}
		} else {
			transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
		return transport
	}
	transport.Proxy = http.ProxyURL(proxyURL)
	return transport
}

func applyManagementProxy(transport *http.Transport) *http.Transport {
	if transport == nil || !managementProxyConfigured() {
		return transport
	}
	return applyProxyURLToTransport(transport, managementProxyURL)
}

func managementTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialContext
	transport.TLSClientConfig = tlsConfigWithRootCAs("")
	applyManagementProxy(transport)
	return transport
}

var managementHTTPClientCache struct {
	sync.RWMutex
	client *http.Client
}

func managementHTTPClient(timeout time.Duration) *http.Client {
	managementHTTPClientCache.RLock()
	client := managementHTTPClientCache.client
	managementHTTPClientCache.RUnlock()
	if client != nil {
		if timeout > 0 {
			copied := *client
			copied.Timeout = timeout
			return &copied
		}
		return client
	}

	transport := managementTransport()
	name := "upstream"
	if managementProxyConfigured() {
		name = "upstream-proxy"
	}
	client = &http.Client{Transport: wrapDebugTransport(name, transport), Timeout: upstreamHTTPClient.Timeout}
	managementHTTPClientCache.Lock()
	managementHTTPClientCache.client = client
	managementHTTPClientCache.Unlock()
	if timeout > 0 {
		copied := *client
		copied.Timeout = timeout
		return &copied
	}
	return client
}

func maskProxyURL(raw *url.URL) string {
	if raw == nil {
		return ""
	}

	return raw.Scheme + "://" + raw.Host
}

func proxySourceLabel() string {
	if managementProxyConfigured() {
		return "程序配置"
	}
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return "系统环境变量"
		}
	}
	return ""
}

func proxyStatusLabel() string {
	if inUse := managementProxyInUse(); inUse != nil {
		return maskProxyURL(inUse)
	}
	return ""
}

func applyManagementProxyFromConfigFile(path string) {
	cfg, _, err := loadOrCreateCLIConfig(path)
	if err != nil {
		return
	}
	if u := strings.TrimSpace(cfg.URL); u != "" {
		speedURLConfigCandidates = append(speedURLConfigCandidates, u)
	}
	if u := strings.TrimSpace(cfg.NSBURL); u != "" {
		speedURLConfigCandidates = append(speedURLConfigCandidates, u)
	}
	if err := setManagementProxy(cfg.Proxy); err != nil {
		fmt.Printf("[proxy] 管理面代理配置无效，未启用: %v\n", err)
	}
}

var configFileWriteMutex sync.Mutex

func updateProxyInConfigFile(path, raw string) error {
	configFileWriteMutex.Lock()
	defer configFileWriteMutex.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {

			if err := writeCLIConfigTemplate(path, defaultCLIFileConfig()); err != nil {
				return err
			}
			data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	var probe map[string]interface{}
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("配置文件不是合法 JSON: %w", err)
	}
	if configField, nested := probe["config"].(map[string]interface{}); nested {
		configField["proxy"] = raw
	} else {
		probe["proxy"] = raw
	}
	out, err := json.MarshalIndent(probe, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if err := atomicWriteFile(path, out, 0600); err != nil {
		return err
	}
	os.WriteFile(path+".bak", data, 0600)
	return nil
}

// ----------------------- 管理面代理连通性测试 -----------------------

const proxyTestTraceURL = "https://www.cloudflare.com/cdn-cgi/trace"
const proxyTestTimeout = 10 * time.Second

// resolveProxyTestTarget 决定代理测试目标：raw 非空时测试该地址本身（来源=程序配置）；
// raw 为空时按生效链回退——程序配置 → 系统环境变量（与 managementProxyInUse 一致）→ 直连。
func resolveProxyTestTarget(raw string) (*url.URL, string, error) {
	value := strings.TrimSpace(raw)
	if value != "" {
		parsed, err := parseManagementProxy(value)
		if err != nil {
			return nil, "", err
		}
		return parsed, "程序配置", nil
	}
	if inUse := managementProxyInUse(); inUse != nil {
		if managementProxyConfigured() {
			return inUse, "程序配置", nil
		}
		return inUse, "系统环境变量", nil
	}
	return nil, "直连", nil
}

// testManagementProxyConnectivity 对目标代理做一次性连通性测试：请求 Cloudflare trace，
// 返回可用性、出口 IP、出口地区、延迟与脱敏代理标签；不修改全局代理状态与配置文件。
func testManagementProxyConnectivity(raw string) map[string]interface{} {
	result := map[string]interface{}{"ok": false, "exitIP": "", "country": "", "latencyMs": int64(0), "proxyLabel": "", "source": "", "error": ""}
	proxyURL, source, err := resolveProxyTestTarget(raw)
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	result["source"] = source
	if proxyURL != nil {
		result["proxyLabel"] = maskProxyURL(proxyURL)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialContext
	transport.TLSClientConfig = tlsConfigWithRootCAs("")
	// 测试目标完全由 resolveProxyTestTarget 决定，显式清空默认的环境变量代理，避免来源标注与实际出口不符
	transport.Proxy = nil
	applyProxyURLToTransport(transport, proxyURL)
	client := &http.Client{Transport: transport, Timeout: proxyTestTimeout}

	req, err := http.NewRequest(http.MethodGet, proxyTestTraceURL, nil)
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	req.Header.Set("User-Agent", "CFData-WEB/"+appVersion)
	start := time.Now()
	resp, err := client.Do(req)
	result["latencyMs"] = time.Since(start).Milliseconds()
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		result["error"] = fmt.Sprintf("检测端点返回状态 %d", resp.StatusCode)
		return result
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	exitIP, country := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "ip":
			exitIP = strings.TrimSpace(val)
		case "loc":
			country = strings.ToUpper(strings.TrimSpace(val))
		}
	}
	if exitIP == "" || country == "" {
		result["error"] = "trace 响应缺少 ip/loc 字段"
		return result
	}
	result["ok"] = true
	result["exitIP"] = exitIP
	result["country"] = country
	return result
}
