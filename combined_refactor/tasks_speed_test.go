package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// 用毫秒级退避替换真实等待，避免测试变慢。
func useFastSpeedBackoff(t *testing.T) {
	t.Helper()
	old := speedRateLimitBackoff
	speedRateLimitBackoff = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { speedRateLimitBackoff = old })
}

func fakeSpeedRunner(sequence func(call int) (float64, string)) speedTestRunner {
	call := 0
	return func(ctx context.Context, ip string, port int, customURL string) (float64, string) {
		call++
		return sequence(call)
	}
}

func TestSpeedRateLimitRetryRecovers(t *testing.T) {
	useFastSpeedBackoff(t)
	calls := 0
	run := func(ctx context.Context, ip string, port int, customURL string) (float64, string) {
		calls++
		if calls < 3 {
			return 0, "测速失败（速率限制）"
		}
		return 12.5, ""
	}
	speed, errText, retried := speedWithRateLimitRetry(context.Background(), run, "1.2.3.4", 443, "src-a")
	if !retried {
		t.Fatal("429 应触发重试")
	}
	if errText != "" {
		t.Fatalf("重试成功后不应保留错误: %q", errText)
	}
	if speed != 12.5 || calls != 3 {
		t.Fatalf("speed=%v calls=%d", speed, calls)
	}
}

func TestSpeedRateLimitRetryGivesUpAfterTwoRetries(t *testing.T) {
	useFastSpeedBackoff(t)
	calls := 0
	run := func(ctx context.Context, ip string, port int, customURL string) (float64, string) {
		calls++
		return 0, "测速失败（速率限制）"
	}
	_, errText, retried := speedWithRateLimitRetry(context.Background(), run, "1.2.3.4", 443, "src-a")
	if !retried || !isSpeedRateLimited(errText) {
		t.Fatalf("应返回限流结果: retried=%v err=%q", retried, errText)
	}
	if calls != 1+len(speedRateLimitBackoff) {
		t.Fatalf("重试次数=%d，期望 %d", calls, 1+len(speedRateLimitBackoff))
	}
}

func TestSpeedRateLimitRetryStopsOnCancel(t *testing.T) {
	old := speedRateLimitBackoff
	speedRateLimitBackoff = []time.Duration{time.Hour}
	defer func() { speedRateLimitBackoff = old }()

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	run := func(c context.Context, ip string, port int, customURL string) (float64, string) {
		calls++
		cancel()
		return 0, "测速失败（速率限制）"
	}
	start := time.Now()
	_, _, retried := speedWithRateLimitRetry(ctx, run, "1.2.3.4", 443, "src-a")
	if !retried || calls != 1 {
		t.Fatalf("取消后不应继续重试: retried=%v calls=%d", retried, calls)
	}
	if time.Since(start) > time.Second {
		t.Fatal("取消后应立即返回，不应等待退避")
	}
}

func TestOfficialSpeedCoreSwitchesSourceAndStopsAfterSix(t *testing.T) {
	useFastSpeedBackoff(t)
	results := make([]TestResult, 8)
	for i := range results {
		results[i] = TestResult{IP: fmt.Sprintf("10.0.0.%d", i+1), Port: 443}
	}
	rateLimitedCalls := 0
	switches := make([][2]string, 0, 1)
	opts := officialSpeedOptions{
		port:       443,
		limit:      20,
		speedMinMB: 0.1,
		speedURL:   "src-a",
		run:        fakeSpeedRunner(func(int) (float64, string) { return 0, "测速失败（速率限制）" }),
		onRateLimited: func() {
			rateLimitedCalls++
		},
		onSourceSwitch: func(from, to string) {
			switches = append(switches, [2]string{from, to})
		},
		switchSource: func(current string) (string, bool) {
			if current == "src-a" {
				return "src-b", true
			}
			return "", false
		},
	}
	updated, qualified := runOfficialSpeedTestsCore(context.Background(), results, opts)
	if len(qualified) != 0 {
		t.Fatalf("全部限流不应有达标项: %d", len(qualified))
	}
	if rateLimitedCalls != 1 {
		t.Fatalf("应在连续限流后终止一次，实际 %d", rateLimitedCalls)
	}
	if len(switches) != 1 || switches[0] != [2]string{"src-a", "src-b"} {
		t.Fatalf("应切换一次测速源: %v", switches)
	}
	processed := 0
	for _, r := range updated {
		if r.Speed != "" {
			processed++
		}
	}
	if processed != officialSpeedMaxConsecutiveRateLimited {
		t.Fatalf("应处理 %d 个 IP 后终止，实际 %d", officialSpeedMaxConsecutiveRateLimited, processed)
	}
}

func TestOfficialSpeedCoreSuccessResetsRateLimitCounter(t *testing.T) {
	useFastSpeedBackoff(t)
	results := make([]TestResult, 9)
	for i := range results {
		results[i] = TestResult{IP: fmt.Sprintf("10.1.0.%d", i+1), Port: 443}
	}
	// 前 5 个全限流，第 6 个成功，随后 3 个继续限流：不应触发终止。
	call := 0
	run := func(ctx context.Context, ip string, port int, customURL string) (float64, string) {
		call++
		if ip == results[5].IP {
			return 50, ""
		}
		return 0, "测速失败（速率限制）"
	}
	rateLimitedCalls := 0
	opts := officialSpeedOptions{
		port:       443,
		limit:      20,
		speedMinMB: 0.1,
		speedURL:   "src-a",
		run:        run,
		onRateLimited: func() {
			rateLimitedCalls++
		},
		switchSource: func(current string) (string, bool) { return "src-b", current != "src-b" },
	}
	_, qualified := runOfficialSpeedTestsCore(context.Background(), results, opts)
	if rateLimitedCalls != 0 {
		t.Fatalf("成功一次后计数应重置，不应终止（实际终止 %d 次）", rateLimitedCalls)
	}
	if len(qualified) != 1 {
		t.Fatalf("应有一个达标项: %d", len(qualified))
	}
}
