package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func shouldWarnProxyCountry(country string) bool {
	country = strings.ToUpper(strings.TrimSpace(country))
	return country == "" || (country != "CN" && country != "XX" && country != "T1")
}

func detectCloudflareTraceCountry(ctx context.Context) (string, bool) {
	var lastErr string
	for attempt := 1; attempt <= 3; attempt++ {
		country, ok, errText := detectCloudflareTraceCountryOnce(ctx)
		if ok {
			return country, true
		}
		lastErr = errText
	}
	if lastErr != "" {
		recordDebugError("proxy_country_check", "三次检测失败: "+lastErr)
	}
	return "", false
}

func detectCloudflareTraceCountryOnce(ctx context.Context) (string, bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.cloudflare.com/cdn-cgi/trace", nil)
	if err != nil {
		return "", false, err.Error()
	}
	req.Header.Set("User-Agent", "CFData-WEB/"+appVersion)
	resp, err := upstreamHTTPClient.Do(req)
	if err != nil {
		recordDebugError("proxy_country_check", err.Error())
		return "", false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		errText := fmt.Sprintf("Cloudflare trace status %d", resp.StatusCode)
		recordDebugError("proxy_country_check", errText)
		return "", false, errText
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		recordDebugError("proxy_country_check", err.Error())
		return "", false, err.Error()
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key == "loc" {
			country := strings.ToUpper(strings.TrimSpace(value))
			return country, country != "", ""
		}
	}
	return "", false, "Cloudflare trace missing loc"
}

