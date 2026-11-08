package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestResolveProxyTestTargetExplicitValue(t *testing.T) {
	u, source, err := resolveProxyTestTarget("socks5://127.0.0.1:1080")
	if err != nil {
		t.Fatalf("resolveProxyTestTarget 返回错误: %v", err)
	}
	if source != "程序配置" {
		t.Fatalf("source = %q, want 程序配置", source)
	}
	if u == nil || u.Scheme != "socks5" || u.Host != "127.0.0.1:1080" {
		t.Fatalf("proxyURL = %+v, want socks5://127.0.0.1:1080", u)
	}
}

func TestResolveProxyTestTargetExplicitValueInvalid(t *testing.T) {
	cases := []string{
		"127.0.0.1:1080",      // 缺协议头
		"ftp://127.0.0.1:21",  // 不支持的协议
		"socks5://",           // 缺主机
		"socks5://127.0.0.1",  // 缺端口
	}
	for _, raw := range cases {
		u, source, err := resolveProxyTestTarget(raw)
		if err == nil {
			t.Fatalf("resolveProxyTestTarget(%q) 应返回错误", raw)
		}
		if u != nil || source != "" {
			t.Fatalf("resolveProxyTestTarget(%q) 失败时应返回零值, got %v / %q", raw, u, source)
		}
	}
}

func TestResolveProxyTestTargetEmptyFallsBack(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://10.0.0.1:7890")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("ALL_PROXY", "")
	u, source, err := resolveProxyTestTarget("")
	if err != nil {
		t.Fatalf("resolveProxyTestTarget 返回错误: %v", err)
	}
	if source != "系统环境变量" {
		t.Fatalf("source = %q, want 系统环境变量", source)
	}
	if u == nil || u.Host != "10.0.0.1:7890" {
		t.Fatalf("proxyURL = %+v, want http://10.0.0.1:7890", u)
	}
}

func TestResolveProxyTestTargetEmptyNoProxyDirect(t *testing.T) {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(key, "")
	}
	// 程序配置也不得残留，保证测试独立性
	savedRaw, savedURL := managementProxyRaw, managementProxyURL
	t.Cleanup(func() { managementProxyRaw, managementProxyURL = savedRaw, savedURL })
	managementProxyRaw, managementProxyURL = "", nil

	u, source, err := resolveProxyTestTarget("")
	if err != nil {
		t.Fatalf("resolveProxyTestTarget 返回错误: %v", err)
	}
	if source != "直连" {
		t.Fatalf("source = %q, want 直连", source)
	}
	if u != nil {
		t.Fatalf("proxyURL = %+v, want nil（直连）", u)
	}
}

func TestResolveProxyTestTargetEmptyPrefersProgramConfig(t *testing.T) {
	savedRaw, savedURL := managementProxyRaw, managementProxyURL
	t.Cleanup(func() { managementProxyRaw, managementProxyURL = savedRaw, savedURL })
	if err := setManagementProxy("socks5h://10.0.0.2:1080"); err != nil {
		t.Fatalf("setManagementProxy: %v", err)
	}
	t.Setenv("HTTPS_PROXY", "http://10.0.0.1:7890")
	u, source, err := resolveProxyTestTarget("")
	if err != nil {
		t.Fatalf("resolveProxyTestTarget 返回错误: %v", err)
	}
	if source != "程序配置" {
		t.Fatalf("source = %q, want 程序配置（程序配置优先于环境变量）", source)
	}
	if u == nil || u.Host != "10.0.0.2:1080" {
		t.Fatalf("proxyURL = %+v, want socks5h://10.0.0.2:1080", u)
	}
}

func TestProxyTestResultMasked(t *testing.T) {
	// 认证信息脱敏：结果中不得出现用户名/密码
	masked := maskProxyURL(parseURLForTest(t, "socks5://alice:secret@10.0.0.3:1080"))
	if strings.Contains(masked, "alice") || strings.Contains(masked, "secret") {
		t.Fatalf("masked = %q, 泄露了认证信息", masked)
	}
	if masked != "socks5://10.0.0.3:1080" {
		t.Fatalf("masked = %q, want socks5://10.0.0.3:1080", masked)
	}
}

func parseURLForTest(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", raw, err)
	}
	return u
}
