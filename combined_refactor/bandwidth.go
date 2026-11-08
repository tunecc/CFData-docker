package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// 总测速带宽限速（学习 subs-check-pro check/check.go:350-356 的 token bucket 与
// calcSpeedConcurrency L/r 推导，见 docs/analyses/subs-check-pro-learnings.md 调查③）：
// 配置 speedbandwidth（MB/s，0=关闭）后，所有并发测速下载共享一个任务级令牌桶，
// 防止并发把出口带宽摊薄到测不准或把本机网络打满；启用时测速并发按带宽预算推导。

const mbPerSecBytes = 1024 * 1024

// tokenBucket 极简单速率令牌桶：按 elapsed 时间持续补充令牌，容量为 rate/10
// （对标 subs-check-pro capacity := int64(rate / 10)），线程安全。
type tokenBucket struct {
	mu       sync.Mutex
	rate     float64 // 每秒可消耗的令牌数（字节）
	capacity float64
	tokens   float64
	last     time.Time
}

func newTokenBucket(bytesPerSecond float64) *tokenBucket {
	if bytesPerSecond <= 0 {
		return nil
	}
	capacity := bytesPerSecond / 10
	if capacity < 1 {
		capacity = 1
	}
	return &tokenBucket{
		rate:     bytesPerSecond,
		capacity: capacity,
		tokens:   capacity,
		last:     time.Now(),
	}
}

// refill 按流逝时间补充令牌；调用方需持有锁。
func (b *tokenBucket) refill(now time.Time) {
	if !now.After(b.last) {
		return
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = min(b.capacity, b.tokens+elapsed*b.rate)
	b.last = now
}

// waitN 阻塞直到取得 n 个令牌；ctx 取消时返回 ctx.Err()。
// n 超过桶容量时按容量分多次等待（测速分块远小于容量，防御性兜底）。
func (b *tokenBucket) waitN(ctx context.Context, n int64) error {
	if b == nil || n <= 0 {
		return nil
	}
	request := float64(n)
	for {
		b.mu.Lock()
		now := time.Now()
		b.refill(now)
		if b.tokens >= request {
			b.tokens -= request
			b.mu.Unlock()
			return nil
		}
		deficit := request - b.tokens
		b.mu.Unlock()

		wait := time.Duration(deficit / b.rate * float64(time.Second))
		if wait < time.Millisecond {
			wait = time.Millisecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// activeSpeedBucket 保存当前任务共享的测速令牌桶；nil 表示不限速。
// 单任务互斥下安全：测速阶段开始时设置，阶段结束清理。
var activeSpeedBucket atomic.Pointer[tokenBucket]

// setSpeedTestBandwidthMB 按配置（MB/s，0=关闭）创建并登记任务级令牌桶，返回清理函数。
func setSpeedTestBandwidthMB(bandwidthMB float64) (stop func()) {
	if bandwidthMB <= 0 {
		activeSpeedBucket.Store(nil)
		return func() { activeSpeedBucket.Store(nil) }
	}
	activeSpeedBucket.Store(newTokenBucket(bandwidthMB * mbPerSecBytes))
	return func() { activeSpeedBucket.Store(nil) }
}

// speedTestBucketWaitN 在测速读取了 n 字节后按量取令牌；未启用限速时空操作。
func speedTestBucketWaitN(ctx context.Context, n int64) error {
	return activeSpeedBucket.Load().waitN(ctx, n)
}

// resolveSpeedThreads 按 L/r 推导测速并发（对标 calcSpeedConcurrency 的带宽预算分支）：
// bandwidthMB<=0 或 threads<=0 时原样返回；否则并发 = min(threads, max(1, int(L/r)))，
// r 为单线程目标吞吐（MB/s，低于 0.1 按 0.1 计）。
func resolveSpeedThreads(threads int, speedMin, bandwidthMB float64) int {
	if bandwidthMB <= 0 || threads <= 0 {
		return threads
	}
	r := speedMin
	if r < 0.1 {
		r = 0.1
	}
	c := int(bandwidthMB / r)
	if c < 1 {
		c = 1
	}
	return min(c, threads)
}
