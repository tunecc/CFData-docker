package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 任务完成 webhook 通知（学习 subs-check-pro 的通知闭环思路的轻量版，
// 见 docs/analyses/subs-check-pro-learnings.md 建议④）：支持 Bark 与
// Telegram 两个渠道，凭据存于服务器配置文件、不落日志明文；
// 发送失败只记录，不阻塞任务收尾。

const (
	notifyHTTPTimeout     = 5 * time.Second
	defaultBarkServerBase = "https://api.day.app"
)

type notifyChannels struct {
	BarkServer string
	BarkKey    string
	TGToken    string
	TGChatID   string
}

func loadNotifyChannels() notifyChannels {
	cfg, err := readScheduleConfigFile(defaultCLIConfigPath())
	if err != nil {
		return notifyChannels{}
	}
	return notifyChannels{
		BarkServer: cfg.NotifyBarkServer,
		BarkKey:    cfg.NotifyBarkKey,
		TGToken:    cfg.NotifyTelegramToken,
		TGChatID:   cfg.NotifyTelegramChatID,
	}
}

func (c notifyChannels) barkEnabled() bool {
	return strings.TrimSpace(c.BarkKey) != ""
}

func (c notifyChannels) telegramEnabled() bool {
	return strings.TrimSpace(c.TGToken) != "" && strings.TrimSpace(c.TGChatID) != ""
}

func (c notifyChannels) barkServerBase() string {
	base := strings.TrimRight(strings.TrimSpace(c.BarkServer), "/")
	if base == "" {
		return defaultBarkServerBase
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return defaultBarkServerBase
	}
	return base
}

// buildTaskNotifyMessage 组装固定格式的结果摘要通知标题与正文。
func buildTaskNotifyMessage(kind, mode, label string, resultCount int, duration time.Duration, detail string) (string, string) {
	modeName := mode
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "official":
		modeName = "官方优选"
	case "nsb":
		modeName = "非标优选"
	}
	var title string
	if kind == "failure" {
		title = "CFData 任务失败"
	} else {
		title = "CFData 任务完成"
	}
	body := fmt.Sprintf("%s · 结果 %d 个 · 用时 %s", modeName, resultCount, formatNotifyDuration(duration))
	if label != "" && label != modeName {
		body = label + " · " + body
	}
	if detail != "" {
		body += "\n" + detail
	}
	return title, body
}

func formatNotifyDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	sec := int(d.Seconds() + 0.5)
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	min := sec / 60
	if min < 60 {
		return fmt.Sprintf("%dm%ds", min, sec%60)
	}
	return fmt.Sprintf("%dh%dm", min/60, min%60)
}

// sendNotification 向所有已配置渠道发送通知；逐渠道独立超时与一次重试，
// 返回各渠道的错误摘要（空字符串表示成功）。
func sendNotification(channels notifyChannels, title, body string) map[string]string {
	results := map[string]string{}
	if channels.barkEnabled() {
		results["bark"] = sendWithRetry("bark", func() error {
			return sendBarkNotification(channels, title, body)
		})
	}
	if channels.telegramEnabled() {
		results["telegram"] = sendWithRetry("telegram", func() error {
			return sendTelegramNotification(channels, title, body)
		})
	}
	return results
}

func sendWithRetry(name string, send func() error) string {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if err = send(); err == nil {
			return ""
		}
		time.Sleep(300 * time.Millisecond)
	}
	log.Printf("[notify] %s 通知发送失败: %v", name, err)
	return err.Error()
}

func notifyHTTPPost(ctx context.Context, target string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := managementHTTPClient(notifyHTTPTimeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func sendBarkNotification(channels notifyChannels, title, body string) error {
	// Bark /push 端点支持 JSON：device_key 走 body，避免中文标题在 URL 中转义
	target := channels.barkServerBase() + "/push"
	payload := map[string]string{
		"device_key": strings.TrimSpace(channels.BarkKey),
		"title":      title,
		"body":       body,
	}
	ctx, cancel := context.WithTimeout(context.Background(), notifyHTTPTimeout)
	defer cancel()
	return notifyHTTPPost(ctx, target, payload)
}

func sendTelegramNotification(channels notifyChannels, title, body string) error {
	token := strings.TrimSpace(channels.TGToken)
	if strings.ContainsAny(token, "/") {
		return fmt.Errorf("telegram token 格式无效")
	}
	target := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", url.PathEscape(token))
	text := title + "\n" + body
	payload := map[string]string{
		"chat_id": strings.TrimSpace(channels.TGChatID),
		"text":    text,
	}
	ctx, cancel := context.WithTimeout(context.Background(), notifyHTTPTimeout)
	defer cancel()
	return notifyHTTPPost(ctx, target, payload)
}

// sendTaskNotifyAsync 在独立 goroutine 中发送任务结果通知，绝不阻塞任务收尾；
// 未配置任何渠道时零请求发出。
func sendTaskNotifyAsync(kind, mode, label string, resultCount int, startedAt time.Time, detail string) {
	channels := loadNotifyChannels()
	if !channels.barkEnabled() && !channels.telegramEnabled() {
		return
	}
	title, body := buildTaskNotifyMessage(kind, mode, label, resultCount, time.Since(startedAt), detail)
	safeGo("notify", nil, func() {
		sendNotification(channels, title, body)
	})
}

// sendTestNotification 向指定渠道发送测试消息，同步返回错误摘要（空为成功）。
func sendTestNotification(channel string) (string, string) {
	channels := loadNotifyChannels()
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "bark":
		if !channels.barkEnabled() {
			return "bark", "未配置 Bark key"
		}
		return "bark", sendWithRetry("bark", func() error {
			return sendBarkNotification(channels, "CFData 测试通知", "这是一条测试消息，收到即说明 Bark 通知配置有效。")
		})
	case "telegram":
		if !channels.telegramEnabled() {
			return "telegram", "未配置 Telegram bot token 或 chat ID"
		}
		return "telegram", sendWithRetry("telegram", func() error {
			return sendTelegramNotification(channels, "CFData 测试通知", "这是一条测试消息，收到即说明 Telegram 通知配置有效。")
		})
	default:
		return channel, "未知通知渠道"
	}
}
