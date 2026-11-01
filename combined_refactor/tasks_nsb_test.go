package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeDNSServersFormats(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"8.8.8.8", []string{"8.8.8.8:53"}},
		{"8.8.8.8,1.1.1.1", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{" 8.8.8.8 ; 1.1.1.1 ", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"dns.google:853", []string{"dns.google:853"}},
		{"[2606:4700:4700::1111]", []string{"[2606:4700:4700::1111]:53"}},
	}
	for _, tc := range cases {
		got := normalizeDNSServers(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("normalizeDNSServers(%q) = %v, 期望 %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("normalizeDNSServers(%q)[%d] = %q, 期望 %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestIsDNSErrorClassification(t *testing.T) {
	if isDNSError(nil) {
		t.Fatal("nil 不是 DNS 错误")
	}
	if isDNSError(errors.New("connection refused")) {
		t.Fatal("普通错误不应分类为 DNS 错误")
	}
	var dnsErr *net.DNSError
	dnsErr = &net.DNSError{Err: "no such host", Name: "example.com", IsNotFound: true}
	if !isDNSError(dnsErr) {
		t.Fatal("net.DNSError 应分类为 DNS 错误")
	}
}

func TestResolveHostToIPsDirectIPAndLimit(t *testing.T) {
	// IP 直填：直接返回，不触网
	got, err := resolveHostToIPs(context.Background(), "1.2.3.4", 0)
	if err != nil || len(got) != 1 || got[0] != "1.2.3.4" {
		t.Fatalf("IP 直填应原样返回，得到 %v, err=%v", got, err)
	}
	got, err = resolveHostToIPs(context.Background(), "2606:4700::1111", 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("IPv6 直填应原样返回，得到 %v, err=%v", got, err)
	}
	// 域名 + maxIPs：解析走系统 DNS；CI 环境可能无网络，仅断言不 panic 且结果满足截断
	got, err = resolveHostToIPs(context.Background(), "localhost", 1)
	if err == nil && len(got) > 1 {
		t.Fatalf("maxIPs=1 应截断结果，得到 %v", got)
	}
}

func TestWriteNSBCSVCompactAndFull(t *testing.T) {
	dir := t.TempDir()
	// writeNSBCSV 经 safeFilename 只保留 basename，切到临时目录验证输出
	oldWd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatal(wdErr)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	out := "ip.csv"
	results := []iptestResult{
		{ipAddr: "1.2.3.4", port: 443, dataCenter: "HKG", latency: "12ms", ipType: "v4", originalInput: "1.2.3.4 443"},
		{ipAddr: "5.6.7.8", port: 2053, dataCenter: "NRT", latency: "55ms", ipType: "v4", originalInput: "5.6.7.8 2053"},
	}
	if err := writeNSBCSV(out, results, 0, true, "tcping"); err != nil {
		t.Fatalf("精简 CSV 写入失败: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, out))
	if readErr != nil {
		t.Fatalf("读取输出失败: %v", readErr)
	}
	text := string(data)
	if !strings.Contains(text, "IP地址,端口号") {
		t.Fatalf("缺少表头: %q", text[:min(120, len(text))])
	}
	if !strings.Contains(text, "1.2.3.4,443") || !strings.Contains(text, "HKG") {
		t.Fatal("缺少结果行")
	}
	// BOM 存在（UTF-8 兼容 Excel）
	if !strings.HasPrefix(text, "\ufeff") {
		t.Fatal("CSV 应以 UTF-8 BOM 开头")
	}

	// 完整表头包含扩展列
	full := filepath.Join(dir, "full.csv")
	if err := writeNSBCSV(full, results[:1], 0, false, "httping"); err != nil {
		t.Fatalf("完整 CSV 写入失败: %v", err)
	}
	fullText, _ := os.ReadFile(full)
	if !strings.Contains(string(fullText), "访问协议") {
		t.Fatal("完整表头应包含扩展列")
	}

	// 空文件名回退为 ip.csv 且可写
	if err := writeNSBCSV("", results, 0, true, "tcping"); err != nil {
		t.Fatalf("空文件名应回退为默认输出: %v", err)
	}
}

func TestRunNSBScanWorkersStopsAtResultLimit(t *testing.T) {
	total := 100
	var executed atomic.Int64
	progressCalls := atomic.Int64{}
	wasCanceled := runNSBScanWorkers(context.Background(), total, 4, 10, func(current, etaSeconds int) {
		progressCalls.Add(1)
	}, func(idx int) int {
		n := executed.Add(1)
		return int(n)
	})
	if wasCanceled {
		t.Fatal("resultLimit 提前结束不是取消")
	}
	if executed.Load() > 15 {
		t.Fatalf("达到 resultLimit=10 后应停止派发，实际执行 %d", executed.Load())
	}
	if progressCalls.Load() == 0 {
		t.Fatal("应有进度回调")
	}
}

func TestRunNSBScanWorkersContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	total := 50
	var executed atomic.Int64
	// work 等待 ctx 取消后立即返回；取消由定时器在另一 goroutine 触发，
	// 避免与调度循环同 goroutine 造成死锁
	timer := time.AfterFunc(100*time.Millisecond, cancel)
	defer timer.Stop()
	wasCanceled := runNSBScanWorkers(ctx, total, 2, 0, func(current, etaSeconds int) {}, func(idx int) int {
		executed.Add(1)
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(5 * time.Second):
			return 0
		}
	})
	if !wasCanceled {
		t.Fatal("取消后应报告 wasCanceled")
	}
	if executed.Load() >= int64(total) {
		t.Fatalf("取消后不应执行完全部任务，实际 %d", executed.Load())
	}
}
