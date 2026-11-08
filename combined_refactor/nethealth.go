package main

import (
	"context"
	"fmt"
	"math/rand/v2"
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
	recheckBudget atomic.Int64 // 拥塞期复核预算（max(20, 总数/20)，任务级共享；0=不可用）
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

// 拥塞期复核抖动等待区间（学习 subs-check-pro check/check.go:770-781：300ms 基础 +
// 最多 500ms 随机抖动后复核一次，避免把拥塞期失败误判为节点死亡）。
const (
	netHealthRecheckBase = 300 * time.Millisecond
	netHealthRecheckSpan = 500 * time.Millisecond
)

// netHealthRecheckJitter 可注入的抖动函数，单测替换以消除随机性。
var netHealthRecheckJitter = defaultRecheckJitter

func defaultRecheckJitter() time.Duration {
	return netHealthRecheckBase + time.Duration(rand.Int64N(int64(netHealthRecheckSpan)+1))
}

// initNetHealthRecheckBudget 初始化任务级复核预算（幂等，仅首次生效）：
// max(20, 总数/20)，对标 subs-check-pro 复核预算设计（避免死节点被整体复测拖慢检测）。
func (w *netHealthWatcher) initRecheckBudget(total int) {
	budget := int64(max(20, total/20))
	w.recheckBudget.CompareAndSwap(0, budget)
}

// takeRecheck 消耗一次复核预算；预算耗尽或未初始化返回 false。
func (w *netHealthWatcher) takeRecheck() bool {
	for {
		current := w.recheckBudget.Load()
		if current <= 0 {
			return false
		}
		if w.recheckBudget.CompareAndSwap(current, current-1) {
			return true
		}
	}
}

// congestionRecheck 在拥塞期间为一次失败申请复核机会：仅在探针处于拥塞判定状态
// 且预算可用时取得预算并完成 300–800ms 抖动等待，返回 true 表示调用方应立即重试
// 一次；false 表示不复核（未拥塞/预算耗尽/未启动探针/ctx 取消）。
func congestionRecheck(ctx context.Context) bool {
	w := activeNetHealth.Load()
	if w == nil || w.isHealthy() {
		return false
	}
	if !w.takeRecheck() {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(netHealthRecheckJitter()):
	}
	return ctx.Err() == nil
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

// startTaskNetHealth 为整个任务启动探针，并按候选总数初始化任务级复核预算，返回 stop 函数。
func startTaskNetHealth(total int) (stop func()) {
	stop = startScanNetHealth()
	if w := activeNetHealth.Load(); w != nil {
		w.initRecheckBudget(total)
	}
	return stop
}

// ensureNetHealth 在无活跃探针时为独立测速任务启动探针并初始化预算（自起自停）；
// 已有探针（编排管线内调用）时借用，不重复启动，返回的 stop 为空操作。
func ensureNetHealth(total int) (stop func()) {
	if activeNetHealth.Load() != nil {
		return func() {}
	}
	w := newNetHealthWatcher(nil)
	if !activeNetHealth.CompareAndSwap(nil, w) {
		return func() {} // 并发下被他人抢占：借用现有探针
	}
	w.initRecheckBudget(total)
	w.start()
	return func() {
		activeNetHealth.CompareAndSwap(w, nil)
		w.stop()
	}
}
