package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

// A4：L/r 推导。bandwidth=10, speedMin=1 → min(threads,10)；speedMin=2.5 → min(threads,4)；
// speedMin<0.1 按 0.1 计；bandwidth=0 或 threads<=0 原样返回。
func TestResolveSpeedThreads(t *testing.T) {
	cases := []struct {
		name      string
		threads   int
		speedMin  float64
		bandwidth float64
		want      int
	}{
		{"带宽10阈值1", 50, 1.0, 10, 10},
		{"带宽10阈值2.5", 50, 2.5, 10, 4},
		{"并发低于推导值", 3, 1.0, 10, 3},
		{"推导值不足1按1", 50, 20, 10, 1},
		{"阈值低于0.1按下限(10/0.1=100被并发截断)", 50, 0, 10, 50},
		{"带宽0不限", 50, 1.0, 0, 50},
		{"带宽0并发0自动", 0, 1.0, 0, 0},
		{"带宽10并发0自动原样", 0, 1.0, 10, 0},
		{"并发负数原样", -1, 1.0, 10, -1},
	}
	for _, c := range cases {
		if got := resolveSpeedThreads(c.threads, c.speedMin, c.bandwidth); got != c.want {
			t.Errorf("%s: resolveSpeedThreads(%d, %v, %v) = %d, want %d", c.name, c.threads, c.speedMin, c.bandwidth, got, c.want)
		}
	}
}

// A3：多 goroutine 共享桶，按速率约束聚合消耗。10MB/s 桶、8 worker × 8 轮 × 128KB
// = 8MB，速率 10MB/s → 下界 ≈ (8MB-1MB 容量)/10MB/s ≈ 0.7s；不应显著快于下界（限速生效），
// 也不应显著慢于 3 倍下界（实现没有过度阻塞）。
func TestTokenBucketSharedRate(t *testing.T) {
	const rateMB = 10
	bucket := newTokenBucket(rateMB * mbPerSecBytes)

	const workers = 8
	const rounds = 8
	const chunkBytes = 128 * 1024
	totalBytes := int64(workers * rounds * chunkBytes)

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				if err := bucket.waitN(context.Background(), chunkBytes); err != nil {
					t.Errorf("waitN: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	minElapsed := time.Duration(float64(totalBytes-int64(bucket.capacity)) / float64(rateMB*mbPerSecBytes) * float64(time.Second))
	if minElapsed < 0 {
		minElapsed = 0
	}
	if elapsed < time.Duration(float64(minElapsed)*0.9) {
		t.Fatalf("聚合消耗过快: elapsed=%v, 下界=%v（限速未生效）", elapsed, minElapsed)
	}
	if elapsed > 3*minElapsed+500*time.Millisecond {
		t.Fatalf("聚合消耗过慢: elapsed=%v, 下界=%v", elapsed, minElapsed)
	}
}

// waitN 尊重 ctx 取消。
func TestTokenBucketWaitNContextCancel(t *testing.T) {
	bucket := newTokenBucket(0.001 * mbPerSecBytes) // 极低速率，等待必然很长
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := bucket.waitN(ctx, 1<<20); err == nil {
		t.Fatal("预期 ctx 超时返回错误")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("ctx 取消未被尊重: %v", time.Since(start))
	}
}

// nil 桶为空操作；setSpeedTestBandwidthMB 登记/清理全局桶。
func TestSpeedTestBucketWaitNNoopAndLifecycle(t *testing.T) {
	stop := setSpeedTestBandwidthMB(0)
	defer stop()
	if activeSpeedBucket.Load() != nil {
		t.Fatal("带宽 0 不应创建桶")
	}
	if err := speedTestBucketWaitN(context.Background(), 1024); err != nil {
		t.Fatalf("未启用限速时应为空操作: %v", err)
	}

	stop = setSpeedTestBandwidthMB(5)
	if activeSpeedBucket.Load() == nil {
		t.Fatal("带宽 5 应创建桶")
	}
	stop()
	if activeSpeedBucket.Load() != nil {
		t.Fatal("stop 后应清理桶")
	}
}
