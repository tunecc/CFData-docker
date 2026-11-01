package main

import (
	"sync"
	"time"
)

// 登录失败限速器：按 IP 的固定窗口计数，防止 Web 认证被无限暴力尝试。
// 仅在认证启用时由登录处理器调用；内存条目有上限（LRU 淘汰最旧）。

const (
	loginFailureWindow   = 10 * time.Minute
	loginFailureMaxCount = 5
	loginLimiterCapacity = 1000
)

type loginFailureEntry struct {
	count       int
	windowStart time.Time
	lastSeen    time.Time
}

type loginRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginFailureEntry
	now     func() time.Time
}

func newLoginRateLimiter() *loginRateLimiter {
	return &loginRateLimiter{
		entries: make(map[string]*loginFailureEntry),
		now:     time.Now,
	}
}

// allowed 报告该 IP 当前是否允许尝试登录。
func (l *loginRateLimiter) allowed(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, exists := l.entries[ip]
	if !exists {
		return true
	}
	if l.now().Sub(entry.windowStart) >= loginFailureWindow {
		// 窗口过期自动恢复
		delete(l.entries, ip)
		return true
	}
	return entry.count < loginFailureMaxCount
}

// recordFailure 记录一次失败；达到阈值后返回 false（后续将被拒绝）。
func (l *loginRateLimiter) recordFailure(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.evictExpiredLocked(now)
	entry, exists := l.entries[ip]
	if !exists || now.Sub(entry.windowStart) >= loginFailureWindow {
		if !exists && len(l.entries) >= loginLimiterCapacity {
			l.evictOldestLocked()
		}
		entry = &loginFailureEntry{windowStart: now}
		l.entries[ip] = entry
	}
	entry.count++
	entry.lastSeen = now
	// 本次失败尝试本身允许发生（返回 true），后续尝试由 allowed() 拒绝
	return entry.count <= loginFailureMaxCount
}

// recordSuccess 登录成功后清零该 IP 的失败计数。
func (l *loginRateLimiter) recordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

func (l *loginRateLimiter) evictExpiredLocked(now time.Time) {
	for ip, entry := range l.entries {
		if now.Sub(entry.windowStart) >= loginFailureWindow {
			delete(l.entries, ip)
		}
	}
}

func (l *loginRateLimiter) evictOldestLocked() {
	var oldestIP string
	var oldest time.Time
	first := true
	for ip, entry := range l.entries {
		if first || entry.lastSeen.Before(oldest) {
			oldestIP = ip
			oldest = entry.lastSeen
			first = false
		}
	}
	if oldestIP != "" {
		delete(l.entries, oldestIP)
	}
}

func (l *loginRateLimiter) remainingAttempts(ip string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, exists := l.entries[ip]
	if !exists || l.now().Sub(entry.windowStart) >= loginFailureWindow {
		return loginFailureMaxCount
	}
	remaining := loginFailureMaxCount - entry.count
	if remaining < 0 {
		return 0
	}
	return remaining
}
