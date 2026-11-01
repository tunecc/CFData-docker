package main

import (
	"fmt"
	"testing"
	"time"
)

func TestLoginLimiterAllowsInitially(t *testing.T) {
	l := newLoginRateLimiter()
	if !l.allowed("1.2.3.4") {
		t.Fatal("首次访问应允许")
	}
	for i := 0; i < loginFailureMaxCount; i++ {
		if !l.recordFailure("1.2.3.4") {
			t.Fatalf("第 %d 次失败不应立即拒绝本次记录", i+1)
		}
	}
	if l.recordFailure("1.2.3.4") {
		t.Fatal("超过阈值后应拒绝")
	}
	if l.allowed("1.2.3.4") {
		t.Fatal("达到阈值后不允许再尝试")
	}
	// 其他 IP 不受影响
	if !l.allowed("5.6.7.8") {
		t.Fatal("其他 IP 不应被连带限制")
	}
}

func TestLoginLimiterWindowExpiryRestores(t *testing.T) {
	l := newLoginRateLimiter()
	current := time.Now()
	l.now = func() time.Time { return current }
	for i := 0; i < loginFailureMaxCount; i++ {
		l.recordFailure("1.2.3.4")
	}
	if l.allowed("1.2.3.4") {
		t.Fatal("窗口内应被拒绝")
	}
	current = current.Add(loginFailureWindow + time.Second)
	if !l.allowed("1.2.3.4") {
		t.Fatal("窗口过期后应恢复允许")
	}
	if !l.recordFailure("1.2.3.4") {
		t.Fatal("窗口过期后计数应重新开始")
	}
}

func TestLoginLimiterSuccessResets(t *testing.T) {
	l := newLoginRateLimiter()
	for i := 0; i < loginFailureMaxCount-1; i++ {
		l.recordFailure("1.2.3.4")
	}
	l.recordSuccess("1.2.3.4")
	if got := l.remainingAttempts("1.2.3.4"); got != loginFailureMaxCount {
		t.Fatalf("成功后计数应清零，剩余 %d", got)
	}
}

func TestLoginLimiterCapacityEvictsOldest(t *testing.T) {
	l := newLoginRateLimiter()
	ip := func(i int) string {
		return fmt.Sprintf("10.%d.%d.%d", (i/65536)%256, (i/256)%256, i%256)
	}
	for i := 0; i < loginLimiterCapacity+50; i++ {
		l.recordFailure(ip(i))
	}
	if len(l.entries) > loginLimiterCapacity {
		t.Fatalf("条目数应受容量上限约束，实际 %d", len(l.entries))
	}
}
