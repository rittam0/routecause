package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type stageResult struct {
	SimulatedClients int     `json:"simulated_clients"`
	ElapsedMS        float64 `json:"elapsed_ms"`
	LatencyP50MS     float64 `json:"latency_p50_ms"`
	LatencyP95MS     float64 `json:"latency_p95_ms"`
	LatencyP99MS     float64 `json:"latency_p99_ms"`
	Throughput       float64 `json:"streams_per_second"`
	ErrorRate        float64 `json:"error_rate"`
	CompletedStreams int64   `json:"completed_streams"`
	PeakRSSBytes     uint64  `json:"peak_server_rss_bytes,omitempty"`
	ServerCPUSeconds float64 `json:"server_cpu_seconds,omitempty"`
}

func main() {
	baseURL := flag.String("url", "http://127.0.0.1:8765", "RouteCause demo server URL")
	stagesFlag := flag.String("stages", "10,100,1000,10000", "comma-separated simulated SSE client stages")
	pid := flag.Int("pid", 0, "optional RouteCause server PID for Linux CPU/RSS sampling")
	spawn := flag.String("spawn", "", "optional RouteCause executable to start in demo mode for this run")
	timeout := flag.Duration("timeout", 30*time.Second, "per-stage timeout")
	flag.Parse()
	stages, err := parseStages(*stagesFlag)
	if err != nil {
		fatal(err)
	}
	if *spawn != "" {
		process, err := spawnServer(*spawn, *baseURL)
		if err != nil {
			fatal(err)
		}
		defer func() {
			if transport, ok := http.DefaultTransport.(*http.Transport); ok {
				transport.CloseIdleConnections()
			}
			_ = process.Process.Signal(os.Interrupt)
			_ = process.Wait()
		}()
		*pid = process.Process.Pid
	}
	results := make([]stageResult, 0, len(stages))
	for _, clients := range stages {
		result, err := runStage(strings.TrimRight(*baseURL, "/"), clients, *pid, *timeout)
		if err != nil {
			fatal(err)
		}
		results = append(results, result)
	}
	output := map[string]any{"client_kind": "simulated_sse_client", "generated_at": time.Now().UTC(), "stages": results}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fatal(err)
	}
}

func spawnServer(binary, baseURL string) (*exec.Cmd, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	port := parsed.Port()
	if parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || port == "" {
		return nil, fmt.Errorf("spawn requires an http://127.0.0.1:PORT URL")
	}
	command := exec.Command(binary, "serve", "--demo", "--no-open", "--port", port, "--max-sse-clients", "10000")
	command.Stdout, command.Stderr = os.Stderr, os.Stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(strings.TrimRight(baseURL, "/") + "/api/v1/capabilities")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return command, nil
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	return nil, fmt.Errorf("spawned server did not become ready")
}

func runStage(baseURL string, clients, pid int, timeout time.Duration) (stageResult, error) {
	body := bytes.NewBufferString(`{"duration":"quick","demo_scenario":"healthy"}`)
	response, err := http.Post(baseURL+"/api/v1/diagnoses", "application/json", body)
	if err != nil {
		return stageResult{}, fmt.Errorf("start stage: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		data, _ := io.ReadAll(response.Body)
		return stageResult{}, fmt.Errorf("start stage: %s: %s", response.Status, data)
	}
	var started struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&started); err != nil {
		return stageResult{}, err
	}
	transport := &http.Transport{MaxIdleConns: clients, MaxIdleConnsPerHost: clients}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: timeout, Transport: transport}
	latencies := make([]float64, clients)
	var completed, failures atomic.Int64
	startCPU, _ := procCPU(pid)
	var peakRSS atomic.Uint64
	doneMonitor := make(chan struct{})
	go monitorRSS(pid, &peakRSS, doneMonitor)
	startedAt := time.Now()
	var wait sync.WaitGroup
	wait.Add(clients)
	for i := 0; i < clients; i++ {
		go func(index int) {
			defer wait.Done()
			began := time.Now()
			request, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/diagnoses/"+started.ID+"/events", nil)
			stream, err := client.Do(request)
			latencies[index] = float64(time.Since(began).Microseconds()) / 1000
			if err != nil {
				failures.Add(1)
				return
			}
			defer stream.Body.Close()
			if stream.StatusCode != http.StatusOK {
				failures.Add(1)
				io.Copy(io.Discard, stream.Body)
				return
			}
			scanner := bufio.NewScanner(stream.Body)
			terminal := false
			for scanner.Scan() {
				if scanner.Text() == "event: completed" {
					terminal = true
				}
			}
			if scanner.Err() != nil || !terminal {
				failures.Add(1)
				return
			}
			completed.Add(1)
		}(i)
	}
	wait.Wait()
	elapsed := time.Since(startedAt)
	close(doneMonitor)
	endCPU, _ := procCPU(pid)
	sort.Float64s(latencies)
	return stageResult{SimulatedClients: clients, ElapsedMS: float64(elapsed.Microseconds()) / 1000,
		LatencyP50MS: percentile(latencies, .50), LatencyP95MS: percentile(latencies, .95), LatencyP99MS: percentile(latencies, .99),
		Throughput: float64(completed.Load()) / elapsed.Seconds(), ErrorRate: float64(failures.Load()) / float64(clients),
		CompletedStreams: completed.Load(), PeakRSSBytes: peakRSS.Load(), ServerCPUSeconds: endCPU - startCPU}, nil
}

func parseStages(value string) ([]int, error) {
	var stages []int
	for _, item := range strings.Split(value, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil || n < 1 || n > 10000 {
			return nil, fmt.Errorf("stages must contain integers from 1 to 10000")
		}
		stages = append(stages, n)
	}
	return stages, nil
}
func percentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1)*q + .5)
	return values[index]
}
func monitorRSS(pid int, peak *atomic.Uint64, done <-chan struct{}) {
	if pid <= 0 {
		return
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if rss, ok := procRSS(pid); ok && rss > peak.Load() {
				peak.Store(rss)
			}
		}
	}
}
func procRSS(pid int) (uint64, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			n, err := strconv.ParseUint(fields[1], 10, 64)
			return n * 1024, err == nil
		}
	}
	return 0, false
}
func procCPU(pid int) (float64, bool) {
	if pid <= 0 {
		return 0, false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(data))
	if len(fields) < 15 {
		return 0, false
	}
	user, _ := strconv.ParseFloat(fields[13], 64)
	system, _ := strconv.ParseFloat(fields[14], 64)
	return (user + system) / 100, true
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "routecause-load:", err); os.Exit(1) }
