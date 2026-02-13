package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultURL       = "https://api.uouin.com/cloudflare.html"
	defaultInterval  = 30 * time.Minute
	defaultThreshold = 200.0
)

var (
	keywordPattern = regexp.MustCompile(`(?i)(?:带宽|bandwidth|bw)\s*[:=]\s*(\d+(?:\.\d+)?)\s*([mg])(?:bps)?`)
	tokenPattern   = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s*([mg])(?:bps)?$`)
	splitPattern   = regexp.MustCompile(`[\s,|;]+`)
)

type NodeStore struct {
	mu        sync.RWMutex
	nodes     []string
	updatedAt time.Time
	lastError string
}

func (s *NodeStore) Set(nodes []string, updatedAt time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nodes = append([]string(nil), nodes...)
	s.updatedAt = updatedAt
	if err != nil {
		s.lastError = err.Error()
		return
	}
	s.lastError = ""
}

func (s *NodeStore) Snapshot() ([]string, time.Time, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.nodes...), s.updatedAt, s.lastError
}

func main() {
	source := defaultURL
	if len(os.Args) > 1 {
		source = os.Args[1]
	}

	store := &NodeStore{}
	startPolling(source, defaultInterval, defaultThreshold, store)

	r := newRouter(store, defaultThreshold)

	if err := r.Run(":8080"); err != nil {
		fmt.Fprintf(os.Stderr, "启动 Gin 服务失败: %v\n", err)
		os.Exit(1)
	}
}

func startPolling(source string, interval time.Duration, thresholdMbps float64, store *NodeStore) {
	runAndStore(source, thresholdMbps, store)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			runAndStore(source, thresholdMbps, store)
		}
	}()
}

func runAndStore(source string, thresholdMbps float64, store *NodeStore) {
	now := time.Now()
	content, err := fetch(source)
	if err != nil {
		store.Set(nil, now, err)
		fmt.Fprintf(os.Stderr, "[%s] 读取数据失败: %v\n", now.Format(time.RFC3339), err)
		return
	}

	filtered := filterLines(content, thresholdMbps)
	store.Set(filtered, now, nil)
	fmt.Printf("[%s] 节点刷新完成，命中 %d 条\n", now.Format(time.RFC3339), len(filtered))
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

func newRouter(store *NodeStore, threshold float64) *gin.Engine {
	r := gin.Default()
	r.GET("/nodes", func(c *gin.Context) {
		nodes, updatedAt, lastErr := store.Snapshot()
		c.JSON(http.StatusOK, gin.H{
			"count":       len(nodes),
			"threshold_m": threshold,
			"updated_at":  updatedAt.Format(time.RFC3339),
			"last_error":  lastErr,
			"nodes":       nodes,
		})
	})
	return r
}
