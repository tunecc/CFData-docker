package main

import (
	"sync"
	"time"
)

// etaWindowSeconds 是 ETA 估算窗口的秒数；窗口内按秒分桶记录完成量，
// 内存 O(1)，百万级完成量也不会累积时间戳（学习 subs-check-pro 的
// 滑动窗口 + 历史速率融合思路的轻量版，见 docs/analyses/subs-check-pro-learnings.md 建议③）。
const etaWindowSeconds = 60

// etaEstimator 是并发安全的扫描速率估算器：record 记录一次完成，
// etaSeconds 在进度节流点用窗口内实际观测速率推算剩余时间。
type etaEstimator struct {
	mu      sync.Mutex
	buckets [etaWindowSeconds]int64
	lastSec int64
	started bool
}

func newEtaEstimator() *etaEstimator {
	return &etaEstimator{}
}

// record 记录一次完成。时间倒流（系统时钟回调）时清空旧数据、按新窗口重新开始。
func (e *etaEstimator) record(now time.Time) {
	sec := now.Unix()
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started || sec < e.lastSec {
		for i := range e.buckets {
			e.buckets[i] = 0
		}
		e.started = true
		e.lastSec = sec
	} else if sec > e.lastSec {
		// 推进窗口：清空 (lastSec, sec] 覆盖的桶，最多清 60 个
		steps := sec - e.lastSec
		if steps > etaWindowSeconds {
			steps = etaWindowSeconds
		}
		for i := int64(1); i <= steps; i++ {
			e.buckets[(e.lastSec+i)%etaWindowSeconds] = 0
		}
		e.lastSec = sec
	}
	e.buckets[sec%etaWindowSeconds]++
}

// etaSeconds 返回按窗口内实际观测速率推算的剩余秒数；
// 样本不足（观测不足 2 秒或窗口为空）时返回 -1 表示未知。
func (e *etaEstimator) etaSeconds(done, total int, now time.Time) int {
	if total <= 0 || done <= 0 {
		return -1
	}
	if done >= total {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return -1
	}
	sec := now.Unix()
	if sec < e.lastSec {
		return -1
	}
	// 单次遍历 60 个桶：累加窗口总量并定位最早的非空秒
	var sum int64
	oldest := int64(-1)
	for i := int64(0); i < etaWindowSeconds; i++ {
		s := e.lastSec - i
		if s < 0 {
			break
		}
		if v := e.buckets[s%etaWindowSeconds]; v > 0 {
			sum += v
			oldest = s
		}
	}
	if sum <= 0 || oldest < 0 {
		return -1
	}
	span := sec - oldest + 1
	if span < 2 {
		return -1
	}
	if span > etaWindowSeconds {
		span = etaWindowSeconds
	}
	rate := float64(sum) / float64(span)
	if rate <= 0 {
		return -1
	}
	eta := int(float64(total-done)/rate + 0.5)
	if eta < 0 {
		eta = 0
	}
	return eta
}
