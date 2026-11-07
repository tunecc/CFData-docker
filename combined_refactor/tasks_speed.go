package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func isTLSPort(port int) bool {
	return port == 443 || port == 2053 || port == 2083 || port == 2087 || port == 2096 || port == 8443
}

func runWindowedSpeedTest(ctx context.Context, ip string, port int, customURL string) (float64, string) {
	scheme := "http"
	if isTLSPort(port) {
		scheme = "https"
	}

	parsedURL, err := parseSpeedTestURL(customURL, scheme)
	if err != nil {
		return 0, "URL解析错误: " + err.Error()
	}

	transport := &http.Transport{
		DialContext: func(c context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(c, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
		},
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfigWithRootCAs(parsedURL.Hostname()),
		DisableCompression:  true,
	}
	client := http.Client{
		Transport: wrapDebugTransport("official-speed", transport),
		Timeout:   15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return 0, "请求构造错误"
	}
	req.Host = parsedURL.Host
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		return 0, "连接错误"
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return 0, formatSpeedHTTPFailure(resp.StatusCode)
	}

	buf := make([]byte, 32*1024)
	var totalBytes int64

	timeout := time.After(6 * time.Second)
	measuredStart := time.Now()

	readerCtx, readerCancel := context.WithCancel(ctx)
	defer readerCancel()

	type readChunk struct {
		n   int
		err error
	}
	chunks := make(chan readChunk, 16)
	readerDone := make(chan struct{})
	safeGo("speed-reader", nil, func() {
		defer close(readerDone)
		for {
			n, err := resp.Body.Read(buf)
			select {
			case chunks <- readChunk{n: n, err: err}:
			case <-readerCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	})

	done := false
	canceled := false
loop:
	for !done {
		select {
		case <-ctx.Done():
			canceled = true
			done = true
		case <-timeout:
			done = true
		case chunk := <-chunks:
			if chunk.n > 0 {
				totalBytes += int64(chunk.n)
			}
			if chunk.err != nil {
				done = true
				break loop
			}
		}
	}

	readerCancel()
	resp.Body.Close()
	<-readerDone

	if canceled {
		return 0, "测速任务已终止"
	}

	if totalBytes <= 0 {
		return 0, "0MB/s"
	}

	duration := time.Since(measuredStart).Seconds()
	if duration == 0 {
		duration = 1
	}
	return float64(totalBytes) / duration / 1024 / 1024, ""
}

func formatSpeedHTTPFailure(statusCode int) string {
	if statusCode == http.StatusTooManyRequests {
		return "测速失败（速率限制）"
	}
	return "测速失败"
}

func runSpeedTest(ctx context.Context, session *appSession, ip string, port int, customURL string) {
	session.sendWSMessage("log", fmt.Sprintf("开始对 IP %s 端口 %d 进行测速...", ip, port))

	speedMB, speedErr := runWindowedSpeedTest(ctx, ip, port, customURL)
	if speedErr != "" {
		if ctx.Err() != nil {
			return
		}
		session.sendWSMessage("speed_test_result", map[string]string{"ip": ip, "speed": speedErr})
		session.sendWSMessage("log", "测速失败: "+speedErr)
		return
	}
	if ctx.Err() != nil {
		return
	}
	speedStr := fmt.Sprintf("%.2fMB/s", speedMB)

	session.sendWSMessage("speed_test_result", map[string]string{"ip": ip, "speed": speedStr})
	session.sendWSMessage("log", fmt.Sprintf("IP %s 测速完成: %s", ip, speedStr))
}

const speedRateLimitMessage = "检测到触发速率限制，请稍后重试或更换网络环境"

// 连续多少个 IP 仍触发限流才终止整批测速；429 时先退避重试并尝试换源。
const officialSpeedMaxConsecutiveRateLimited = 6

// 单个 IP 命中 429 后的重试等待序列（测试可覆盖）。
var speedRateLimitBackoff = []time.Duration{3 * time.Second, 6 * time.Second}

func isSpeedRateLimited(speedErr string) bool {
	return strings.Contains(speedErr, "速率限制")
}

// sleepCtx 等待指定时长；任务取消时立即返回 false。
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

type speedTestRunner func(ctx context.Context, ip string, port int, customURL string) (float64, string)

type officialSpeedOptions struct {
	port           int
	limit          int
	speedMinMB     float64
	speedURL       string
	run            speedTestRunner
	onResult       func(current, total, qualified int, result TestResult)
	onRateLimited  func()
	onSourceSwitch func(from, to string)
	switchSource   func(current string) (string, bool)
}

// speedWithRateLimitRetry 执行一次测速；遇到 429 按退避序列重试，返回最后一次结果。
func speedWithRateLimitRetry(ctx context.Context, run speedTestRunner, ip string, port int, speedURL string) (float64, string, bool) {
	speedMB, speedErr := run(ctx, ip, port, speedURL)
	if !isSpeedRateLimited(speedErr) {
		return speedMB, speedErr, false
	}
	for _, wait := range speedRateLimitBackoff {
		if !sleepCtx(ctx, wait) {
			return speedMB, speedErr, true
		}
		speedMB, speedErr = run(ctx, ip, port, speedURL)
		if !isSpeedRateLimited(speedErr) {
			return speedMB, speedErr, true
		}
	}
	return speedMB, speedErr, true
}

func runOfficialSpeedTestsCore(ctx context.Context, results []TestResult, opts officialSpeedOptions) ([]TestResult, []TestResult) {
	run := opts.run
	if run == nil {
		run = runWindowedSpeedTest
	}
	capacity := len(results)
	if opts.limit > 0 && opts.limit < capacity {
		capacity = opts.limit
	}
	qualified := make([]TestResult, 0, capacity)
	interSpeedPause := func() bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(1200 * time.Millisecond):
			return true
		}
	}
	consecutiveRateLimited := 0
	currentURL := opts.speedURL
	for i := range results {
		select {
		case <-ctx.Done():
			return results, qualified
		default:
		}
		if opts.limit > 0 && len(qualified) >= opts.limit {
			break
		}
		speedMB, speedErr, retried := speedWithRateLimitRetry(ctx, run, results[i].IP, opts.port, currentURL)
		if ctx.Err() != nil {
			return results, qualified
		}
		if retried && isSpeedRateLimited(speedErr) && opts.switchSource != nil {
			if next, ok := opts.switchSource(currentURL); ok && next != "" && next != currentURL {
				if opts.onSourceSwitch != nil {
					opts.onSourceSwitch(currentURL, next)
				}
				currentURL = next
			}
		}
		if speedErr != "" {
			results[i].Speed = speedErr
			if isSpeedRateLimited(speedErr) {
				consecutiveRateLimited++
			} else {
				consecutiveRateLimited = 0
			}
		} else {
			consecutiveRateLimited = 0
			results[i].Speed = fmt.Sprintf("%.2fMB/s", speedMB)
			if speedMB >= opts.speedMinMB {
				qualified = append(qualified, results[i])
			}
		}
		if opts.onResult != nil {
			if ctx.Err() != nil {
				return results, qualified
			}
			opts.onResult(i+1, len(results), len(qualified), results[i])
		}
		if !interSpeedPause() {
			return results, qualified
		}
		if consecutiveRateLimited >= officialSpeedMaxConsecutiveRateLimited {
			if opts.onRateLimited != nil {
				opts.onRateLimited()
			}
			break
		}
	}
	return results, qualified
}

func runOfficialSpeedBatch(ctx context.Context, session *appSession, port int, customURL string, speedLimit int, speedMin float64, fallbackResults []TestResult, skipTested bool) {
	if port <= 0 {
		port = 443
	}
	if speedLimit <= 0 {
		session.sendWSMessage("log", "官方批量测速已关闭（测速上限 0）")
		session.sendWSMessage("official_speed_complete", map[string]interface{}{"qualified": 0, "limit": speedLimit, "rateLimited": false})
		return
	}
	if speedMin <= 0 {
		speedMin = 0.1
	}

	session.testMutex.Lock()
	results := append([]TestResult(nil), session.testResults...)
	session.testMutex.Unlock()
	if len(results) == 0 && len(fallbackResults) > 0 {
		results = append([]TestResult(nil), fallbackResults...)
		session.testMutex.Lock()
		session.testResults = append([]TestResult(nil), results...)
		session.testMutex.Unlock()
	}
	if len(results) == 0 {
		session.sendWSMessage("log", "没有可用的详细测试结果，跳过官方批量测速")
		session.sendWSMessage("official_speed_complete", map[string]interface{}{"qualified": 0, "limit": speedLimit, "rateLimited": false})
		return
	}
	if skipTested {
		pending := results[:0]
		for _, result := range results {
			speedText := strings.TrimSpace(result.Speed)
			if speedText == "" || speedText == "未测速" {
				pending = append(pending, result)
			}
		}
		results = pending
		if len(results) == 0 {
			session.sendWSMessage("log", "没有未测速的详细测试结果，跳过继续测速")
			session.sendWSMessage("official_speed_complete", map[string]interface{}{"qualified": 0, "limit": speedLimit, "rateLimited": false})
			return
		}
	}
	sortOfficialTestResults(results)

	session.sendWSMessage("log", fmt.Sprintf("开始测速：%d 条记录，目标上限=%d，测速阈值=%.2fMB/s", len(results), speedLimit, speedMin))
	session.sendWSMessage("official_speed_progress", map[string]interface{}{"current": 0, "total": len(results), "qualified": 0, "limit": speedLimit})
	rateLimited := false
	sourceSwitched := false
	triedSources := map[string]bool{}
	updated, qualified := runOfficialSpeedTestsCore(ctx, results, officialSpeedOptions{
		port:       port,
		limit:      speedLimit,
		speedMinMB: speedMin,
		speedURL:   customURL,
		onResult: func(current, total, qualified int, result TestResult) {
			session.sendWSMessage("speed_test_result", map[string]string{"ip": result.IP, "speed": result.Speed})
			session.sendWSMessage("official_speed_progress", map[string]interface{}{"current": current, "total": total, "qualified": qualified, "limit": speedLimit})
		},
		onRateLimited: func() {
			rateLimited = true
			session.sendWSMessage("log", speedRateLimitMessage)
		},
		onSourceSwitch: func(from, to string) {
			sourceSwitched = true
			session.sendWSMessage("log", fmt.Sprintf("测速源触发限流，已切换测速源：%s → %s", speedURLLabel(from), speedURLLabel(to)))
		},
		switchSource: func(current string) (string, bool) {
			if current != "" {
				triedSources[current] = true
			}
			for _, candidate := range speedURLCandidates() {
				if candidate == current || triedSources[candidate] {
					continue
				}
				if available, known := isSpeedURLAvailable(candidate); known && !available {
					triedSources[candidate] = true
					continue
				}
				return candidate, true
			}
			return "", false
		},
	})

	session.testMutex.Lock()
	byIP := make(map[string]string, len(updated))
	for _, result := range updated {
		byIP[result.IP] = result.Speed
	}
	for idx := range session.testResults {
		if speed, ok := byIP[session.testResults[idx].IP]; ok {
			session.testResults[idx].Speed = speed
		}
	}
	totalQualified := countOfficialQualifiedResults(session.testResults, speedMin)
	session.testMutex.Unlock()
	if ctx.Err() != nil {
		session.sendWSMessage("log", "官方批量测速已终止")
	} else if rateLimited {
		message := fmt.Sprintf("官方批量测速已因速率限制停止，达标 %d/%d，总达标 %d", len(qualified), speedLimit, totalQualified)
		if sourceSwitched {
			message += "（已尝试更换测速源）"
		}
		session.sendWSMessage("log", message)
	} else {
		session.sendWSMessage("log", fmt.Sprintf("官方批量测速完成，达标 %d/%d，总达标 %d", len(qualified), speedLimit, totalQualified))
	}
	if ctx.Err() == nil {
		session.sendWSMessage("official_speed_complete", map[string]interface{}{"qualified": len(qualified), "totalQualified": totalQualified, "limit": speedLimit, "rateLimited": rateLimited})
		session.testMutex.Lock()
		rows := officialResultRows(nil, append([]TestResult(nil), session.testResults...), "")
		session.testMutex.Unlock()
		triggerManualR2Upload(session, speedUploadRows(rows, speedMin, speedLimit))
	}
}

// officialSpeedAttempted 判断本次批量测速是否已经产出过任何测速结果。
// 速度字段为空或「未测速」表示尚未测速；一旦有结果（含失败），取消后不再由调用方补传。
func officialSpeedAttempted(session *appSession) bool {
	session.testMutex.Lock()
	defer session.testMutex.Unlock()
	for _, result := range session.testResults {
		text := strings.TrimSpace(result.Speed)
		if text != "" && text != "未测速" {
			return true
		}
	}
	return false
}

func countOfficialQualifiedResults(results []TestResult, speedMin float64) int {
	count := 0
	for _, result := range results {
		if speed, ok := parseSpeedMBForSort(result.Speed); ok && speed >= speedMin {
			count++
		}
	}
	return count
}
