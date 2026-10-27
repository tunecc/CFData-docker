package main

import (
	"context"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var upstreamHTTPClient = &http.Client{Timeout: 20 * time.Second}

const dataFetchTimeout = 8 * time.Second

//go:embed fallback/locations.snapshot.json
var fallbackLocationsJSON []byte

//go:embed fallback/ips-v4.snapshot.txt
var fallbackIPv4List string

//go:embed fallback/ips-v6.snapshot.txt
var fallbackIPv6List string

func fallbackData(filename string) (string, bool) {
	switch filepath.Base(filename) {
	case "locations.json":
		return string(fallbackLocationsJSON), true
	case "ips-v4.txt":
		return fallbackIPv4List, true
	case "ips-v6.txt":
		return fallbackIPv6List, true
	}
	return "", false
}

func configureHTTPClients() {
	initCustomResolver()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialContext
	transport.TLSClientConfig = tlsConfigWithRootCAs("")
	upstreamHTTPClient.Transport = wrapDebugTransport("upstream", transport)
}

func initLocations() {
	filename := "locations.json"
	url := "https://www.baipiao.eu.org/cloudflare/locations"

	if body, err := os.ReadFile(filename); err == nil {
		if err := loadLocationMap(body); err == nil {
			return
		} else {
			sendLog(fmt.Sprintf("本地 %s 无效（%v），尝试重新获取", filename, err))
		}
	} else if !os.IsNotExist(err) {
		sendLog("读取位置信息失败: " + err.Error())
	} else {
		sendLog("本地 locations.json 不存在，正在从服务器下载...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), dataFetchTimeout)
	content, fetchErr := getURLContentViaProxy(ctx, url)
	cancel()
	if fetchErr == nil {
		if err := loadLocationMap([]byte(content)); err == nil {
			if err := saveToFile(filename, content); err != nil {
				sendLog("保存位置信息失败: " + err.Error())
			}
			return
		} else {
			fetchErr = fmt.Errorf("在线位置数据无效: %w", err)
		}
	}
	recordDebugError("locations_fetch", fetchErr.Error())

	fallback, ok := fallbackData(filename)
	if ok {
		if err := loadLocationMap([]byte(fallback)); err == nil {
			sendLog("已使用内置位置数据（离线快照）")
			if err := saveToFile(filename, fallback); err != nil {
				sendLog("保存内置位置数据失败: " + err.Error())
			}
			return
		} else {
			sendLog("内置位置数据无效: " + err.Error())
		}
	}

	sendLog("获取位置信息失败: " + fetchErr.Error())
	sendLog("⚠️ locationMap 未就绪，后续扫描结果的 城市/地区 列将为空；可删除本地缓存（Web 端右上角设置）或检查网络后重启")
}

func loadLocationMap(content []byte) error {
	locations, err := parseLocationsContent(content)
	if err != nil {
		return err
	}
	next := make(map[string]location)
	for _, loc := range locations {
		if strings.TrimSpace(loc.Iata) == "" {
			continue
		}
		next[loc.Iata] = loc
	}
	locationMap = next
	fmt.Printf("已加载 %d 个数据中心位置信息\n", len(locationMap))
	return nil
}

func parseLocationsContent(content []byte) ([]location, error) {
	var locations []location
	if err := json.Unmarshal(content, &locations); err != nil {
		return nil, err
	}
	valid := 0
	for _, loc := range locations {
		if strings.TrimSpace(loc.Iata) != "" {
			valid++
		}
	}
	if valid == 0 {
		return nil, fmt.Errorf("位置数据为空或缺少 IATA")
	}
	return locations, nil
}

func sendLog(msg string) {
	recordDebugNotice("terminal_log", msg)
	fmt.Println(msg)
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}
var utf8BOMString = string(utf8BOM)

func writeUTF8BOM(f *os.File) error {
	_, err := f.Write(utf8BOM)
	return err
}

func resetAllConfigFiles(session *appSession) {
	configResetMutex.Lock()
	defer configResetMutex.Unlock()

	result := resetConfigResult{
		Success:  true,
		Deleted:  []string{},
		Missing:  []string{},
		Failed:   []string{},
		Reminder: "删掉之后，下次重新运行程序时会重新下载或使用内置离线快照。",
	}

	session.taskMutex.Lock()
	hasRunningTasks := session.isTaskRunning
	session.taskMutex.Unlock()
	if !hasRunningTasks {
		hasRunningTasks = anyTaskRunning()
	}
	if hasRunningTasks {
		result.Success = false
		result.Failed = []string{"当前还有任务在运行（包含其他连接）"}
		session.sendWSMessage("reset_config_result", result)
		session.sendWSMessage("log", "现在还有任务在跑，先停掉再清理本地缓存文件吧")
		return
	}

	resetASNReader()
	files := []string{"ips-v6.txt", "ips-v4.txt", "locations.json", asnDBFile}
	deleted := []string{}
	missing := []string{}
	failed := []string{}

	for _, filename := range files {
		if err := os.Remove(filename); err == nil {
			deleted = append(deleted, filename)
			continue
		} else if os.IsNotExist(err) {
			missing = append(missing, filename)
		} else {
			failed = append(failed, fmt.Sprintf("%s (%v)", filename, err))
		}
	}

	if len(failed) > 0 {
		result.Success = false
	}
	result.Deleted = deleted
	result.Missing = missing
	result.Failed = failed

	session.sendWSMessage("reset_config_result", result)
	if len(failed) > 0 {
		session.sendWSMessage("log", fmt.Sprintf("本地缓存清理失败: %v", failed))
	} else {
		session.sendWSMessage("log", fmt.Sprintf("本地缓存已清理，已删除: %v，原本不存在: %v", deleted, missing))
	}
	session.sendWSMessage("log", "提示：删掉之后，下次重新运行程序时会重新下载或使用内置离线快照。")
}

func getIPListContent(filename, apiURL string) (string, error) {
	if content, err := os.ReadFile(filename); err == nil {
		if err := validateIPListContent(filename, string(content)); err == nil {
			return string(content), nil
		} else {
			sendLog(fmt.Sprintf("本地 %s 无效（%v），尝试重新获取", filename, err))
		}
	} else if !os.IsNotExist(err) {
		sendLog(fmt.Sprintf("读取 %s 失败: %v", filename, err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), dataFetchTimeout)
	content, fetchErr := getURLContentViaProxy(ctx, apiURL)
	cancel()
	if fetchErr == nil {
		if err := validateIPListContent(filename, content); err == nil {
			if err := saveToFile(filename, content); err != nil {
				sendLog("保存 IP 列表缓存失败: " + err.Error())
			}
			return content, nil
		} else {
			fetchErr = fmt.Errorf("在线 %s 数据无效: %w", filename, err)
		}
	}
	recordDebugError("ip_list_fetch", fetchErr.Error())

	fallback, ok := fallbackData(filename)
	if !ok {
		return "", fetchErr
	}
	if err := validateIPListContent(filename, fallback); err != nil {
		return "", fmt.Errorf("内置 %s 数据无效: %w", filename, err)
	}
	if err := saveToFile(filename, fallback); err != nil {
		sendLog("保存内置 IP 列表失败: " + err.Error())
	}
	sendLog(fmt.Sprintf("获取 %s 失败，已使用内置 IP 列表（离线快照）: %v", filename, fetchErr))
	return fallback, nil
}

func validateIPListContent(filename, content string) error {
	entries, err := parseIPList(content)
	if err != nil {
		return err
	}
	wantIPv4 := filepath.Base(filename) == "ips-v4.txt"
	wantIPv6 := filepath.Base(filename) == "ips-v6.txt"
	valid := 0
	for _, entry := range entries {
		ip, _, err := net.ParseCIDR(entry)
		if err != nil {
			continue
		}
		if wantIPv4 && ip.To4() == nil {
			continue
		}
		if wantIPv6 && ip.To4() != nil {
			continue
		}
		valid++
	}
	if valid == 0 {
		return fmt.Errorf("%s 中没有有效的 %s 条目", filename, ipFamilyLabel(filename))
	}
	return nil
}

func ipFamilyLabel(filename string) string {
	switch filepath.Base(filename) {
	case "ips-v4.txt":
		return "IPv4"
	case "ips-v6.txt":
		return "IPv6"
	default:
		return "IP"
	}
}

func getURLContent(targetURL string) (string, error) {
	return getURLContentWithContext(context.Background(), targetURL)
}

func getURLContentWithContext(ctx context.Context, targetURL string) (string, error) {
	data, err := getURLBytesWithContext(ctx, targetURL)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getURLBytes(targetURL string) ([]byte, error) {
	return getURLBytesWithContext(context.Background(), targetURL)
}

func getURLBytesWithContext(ctx context.Context, targetURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := upstreamHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("请求失败: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func getURLBytesWithContextClient(ctx context.Context, targetURL string, client *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("请求失败: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func getURLBytesViaProxy(ctx context.Context, targetURL string) ([]byte, error) {
	return getURLBytesWithContextClient(ctx, targetURL, managementHTTPClient(0))
}

func getURLContentViaProxy(ctx context.Context, targetURL string) (string, error) {
	data, err := getURLBytesViaProxy(ctx, targetURL)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getFileContent(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func saveToFile(filename, content string) error {
	return atomicWriteFile(filename, []byte(content), 0644)
}

func atomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(filename)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filename); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func parseCSVFile(filename string) ([]string, [][]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}

	headers := rows[0]
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], utf8BOMString)
	}
	return headers, rows[1:], nil
}
