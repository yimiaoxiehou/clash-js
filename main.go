package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultURL      = "https://api.uouin.com/cloudflare.html"
	defaultInterval = 30 * time.Minute
)

var (
	keywordPattern = regexp.MustCompile(`(?i)(?:带宽|bandwidth|bw)\s*[:=]\s*(\d+(?:\.\d+)?)\s*([mg])(?:bps)?`)
	tokenPattern   = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s*([mg])(?:bps)?$`)
	splitPattern   = regexp.MustCompile(`[\s,|;]+`)
)

func main() {
	source := defaultURL
	if len(os.Args) > 1 {
		source = os.Args[1]
	}

	runOnce(source)

	ticker := time.NewTicker(defaultInterval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-ticker.C:
			runOnce(source)
		case sig := <-sigCh:
			fmt.Printf("收到信号 %s，程序退出\n", sig)
			return
		}
	}
}

func runOnce(source string) {
	fmt.Printf("[%s] 开始拉取节点数据\n", time.Now().Format(time.RFC3339))
	content, err := fetch(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取数据失败: %v\n", err)
		return
	}

	filtered := filterLines(content, 200)
	if len(filtered) == 0 {
		fmt.Println("未找到带宽大于 200M 的节点")
		return
	}

	for _, line := range filtered {
		fmt.Println(line)
	}
}

func fetch(url string) (string, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP 状态码异常: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func filterLines(content string, thresholdMbps float64) []string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	result := make([]string, 0)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		bw, ok := extractBandwidthMbps(line)
		if !ok || bw <= thresholdMbps {
			continue
		}
		result = append(result, line)
	}

	return result
}

func extractBandwidthMbps(line string) (float64, bool) {
	if m := keywordPattern.FindStringSubmatch(line); len(m) == 3 {
		return convertToMbps(m[1], m[2])
	}

	parts := splitPattern.Split(line, -1)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if m := tokenPattern.FindStringSubmatch(part); len(m) == 3 {
			return convertToMbps(m[1], m[2])
		}
	}

	return 0, false
}

func convertToMbps(number, unit string) (float64, bool) {
	v, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, false
	}

	switch strings.ToLower(unit) {
	case "g":
		return v * 1024, true
	case "m":
		return v, true
	default:
		return 0, false
	}
}
