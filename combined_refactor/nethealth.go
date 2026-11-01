package main

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// 轻量网络健康探针（学习 subs-check-pro 的 NetGuard 思路，见
// docs/analyses/subs-check-pro-learnings.md 建议⑧）：扫描任务期间周期直连
// 探测公共 DNS，连续拥塞时让 worker 在取任务前放慢节奏并输出一次告警，
// 恢复后自动停止节流。任务不中断、结果不受影响。

const (
	netHealthProbeInterval = 15 * time.Second
	netHealthProbeTimeout  = 2 * time.Second
	// 连续多少轮探测全部失败判定为本机网络拥塞
	netHealthCongestionThreshold = 3
	// 拥塞时 worker 取任务前的节流延迟
	netHealthThrottleDelay = 2 * time.Second
)

var netHealthProbeTargets = []string{"223.5.5.5:53", "119.29.29.29:53"}

// netHealthProber 抽象单次探测，便于单测注入。
type netHealthProber func(target string, timeout time.Duration) error

func defaultNetHealthProber(target string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// netHealthWatcher 是并发安全的网络健康状态机。
type netHealthWatcher struct {
	prober        netHealthProber
	interval      time.Duration
	healthy       atomic.Bool
	failureStreak atomic.Int64
	announced     atomic.Bool
	stopCh        chan struct{}
	stopDone      chan struct{}
	startOnce     sync.Once
	probeTarget   string // 最近一轮实际使用的目标（诊断用）
	mu            sync.Mutex
}

func newNetHealthWatcher(prober netHealthProber) *netHealthWatcher {
	if prober == nil {
		prober = defaultNetHealthProber
	}
	return &netHealthWatcher{
		prober:   prober,
		interval: netHealthProbeInterval,
		stopCh:   make(chan struct{}),
		stopDone: make(chan struct{}),
	}
}

// start 启动周期探测；先同步探测一轮，避免任务初期误判。
func (w *netHealthWatcher) start() {
	w.probeOnce()
	w.startOnce.Do(func() {
		go func() {
			defer close(w.stopDone)
			ticker := time.NewTicker(w.interval)
			defer ticker.Stop()
			for {
				select {
				case <-w.stopCh:
					return
				case <-ticker.C:
					w.probeOnce()
				}
			}
		}()
	})
}

func (w *netHealthWatcher) stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
	select {
	case <-w.stopDone:
	case <-time.After(3 * time.Second):
	}
}

func (w *netHealthWatcher) probeOnce() {
	var lastErr error
	reachable := false
	for _, target := range netHealthProbeTargets {
		if err := w.prober(target, netHealthProbeTimeout); err == nil {
			reachable = true
			w.mu.Lock()
			w.probeTarget = target
			w.mu.Unlock()
			break
		} else {
			lastErr = err
		}
	}
	w.mu.Lock()
	if !reachable && w.probeTarget == "" && lastErr != nil {
		// 全部目标失败时记录最后一个目标，便于诊断
		w.probeTarget = "unreachable"
	}
	w.mu.Unlock()
	if reachable {
		w.failureStreak.Store(0)
		w.healthy.Store(true)
		w.announced.Store(false)
		return
	}
	w.healthy.Store(false)
	streak := w.failureStreak.Add(1)
	if streak >= netHealthCongestionThreshold && !w.announced.Swap(true) {
		fmt.Printf("[nethealth] 本机网络疑似拥塞（直连探测连续 %d 轮失败），已自动放慢扫描节奏，任务继续；恢复后自动停止节流。最后错误: %v\n", streak, lastErr)
	}
}

// isHealthy 返回当前是否健康。语义：从未判定拥塞（failureStreak 未达阈值）
// 期间一律视为健康，避免任务初期探测抖动造成误节流。
func (w *netHealthWatcher) isHealthy() bool {
	if w.failureStreak.Load() < netHealthCongestionThreshold {
		return true
	}
	return w.healthy.Load()
}

// currentNetHealthThrottle 返回 worker 取任务前应等待的时长（拥塞时 2s，否则 0）。
func currentNetHealthThrottle() time.Duration {
	w := activeNetHealth.Load()
	if w == nil || w.isHealthy() {
		return 0
	}
	return netHealthThrottleDelay
}

// activeNetHealth 保存当前扫描任务的网络探针；nil 表示无任务探针（不节流）。
var activeNetHealth atomic.Pointer[netHealthWatcher]

// startScanNetHealth 为扫描任务启动探针并注册为活跃探针，返回 stop 函数。
func startScanNetHealth() (stop func()) {
	w := newNetHealthWatcher(nil)
	activeNetHealth.Store(w)
	w.start()
	return func() {
		activeNetHealth.CompareAndSwap(w, nil)
		w.stop()
	}
}
