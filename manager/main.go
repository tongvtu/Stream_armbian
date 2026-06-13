package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed templates/index.html
var templatesFS embed.FS

// Global logs buffer
var (
	logMu   sync.Mutex
	logBuf  []string
	maxLogs = 120
)

// Global CPU percentage calculated in background
var (
	globalCpuPercent int
	cpuMu            sync.Mutex
)

// Logs logging helper
func logMsg(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Println(msg)

	logMu.Lock()
	defer logMu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	logLine := fmt.Sprintf("[%s] %s", timestamp, msg)
	logBuf = append(logBuf, logLine)
	if len(logBuf) > maxLogs {
		logBuf = logBuf[1:]
	}
}

// Structs for API response
type BatteryInfo struct {
	Level       int     `json:"level"`
	Temperature float64 `json:"temperature"`
	Status      string  `json:"status"`
}

type AndroidStatus struct {
	Connected bool        `json:"connected"`
	Model     string      `json:"model"`
	OSVersion string      `json:"os_version"`
	Battery   BatteryInfo `json:"battery"`
}

type HostStatus struct {
	CPU  int `json:"cpu"`
	RAM  int `json:"ram"`
	Disk int `json:"disk"`
}

type TelemetryResponse struct {
	Android      AndroidStatus `json:"android"`
	Host         HostStatus    `json:"host"`
	StreamActive bool          `json:"stream_active"`
}

func main() {
	logMsg("StreamArmbian Manager khởi động...")

	// 1. Start ADB background worker
	go startAdbWorker()

	// 2. Start CPU utilization worker
	go startCpuWorker()

	// 3. Define HTTP routes
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/control", handleControl)
	http.HandleFunc("/api/logs", handleLogs)

	// 4. Run HTTP server on port 5000
	port := ":5000"
	logMsg("Web Dashboard chạy tại http://0.0.0.0%s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Không thể khởi động Web Server: %v", err)
	}
}

// Serve embedded dashboard
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := templatesFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "Không tìm thấy file giao diện", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// Serve JSON telemetry stats
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Phương thức không hợp lệ", http.StatusMethodNotAllowed)
		return
	}

	res := TelemetryResponse{
		Android:      getAndroidStatus(),
		Host:         getHostStatus(),
		StreamActive: isStreamActive(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// Serve control ADB actions
func handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Phương thức không hợp lệ", http.StatusMethodNotAllowed)
		return
	}

	type ControlRequest struct {
		Action string `json:"action"`
	}
	var req ControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Dữ liệu không hợp lệ", http.StatusBadRequest)
		return
	}

	var err error
	var output []byte

	switch req.Action {
	case "wake":
		logMsg("Control: Gửi lệnh đánh thức màn hình điện thoại...")
		output, err = exec.Command("adb", "shell", "input", "keyevent", "224").CombinedOutput()
	case "sleep":
		logMsg("Control: Gửi lệnh tắt màn hình điện thoại...")
		output, err = exec.Command("adb", "shell", "input", "keyevent", "223").CombinedOutput()
	case "reboot":
		logMsg("Control: Khởi động lại điện thoại Android...")
		output, err = exec.Command("adb", "reboot").CombinedOutput()
	case "tunnel":
		logMsg("Control: Thiết lập lại kết nối ADB reverse tunnels...")
		setupAdbReverseTunnels()
	default:
		http.Error(w, "Hành động không được hỗ trợ", http.StatusBadRequest)
		return
	}

	status := "Thành công"
	if err != nil {
		status = fmt.Sprintf("Thất bại: %v (%s)", err, strings.TrimSpace(string(output)))
		logMsg("Control: Thực thi '%s' thất bại: %s", req.Action, status)
	} else {
		logMsg("Control: Thực thi '%s' thành công.", req.Action)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

// Serve rolling log lines
func handleLogs(w http.ResponseWriter, r *http.Request) {
	logMu.Lock()
	defer logMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"logs": strings.Join(logBuf, "\n"),
	})
}

// ADB reverse port forwarding configuration
func setupAdbReverseTunnels() {
	logMsg("ADB: Thiết lập reverse tunnel cho cổng RTMP (1935) và RTSP (8554)...")
	out1, err1 := exec.Command("adb", "reverse", "tcp:1935", "tcp:1935").CombinedOutput()
	if err1 != nil {
		logMsg("ADB Lỗi Reverse RTMP (1935): %v (%s)", err1, strings.TrimSpace(string(out1)))
	} else {
		logMsg("ADB: Thiết lập Reverse RTMP (1935) thành công.")
	}

	out2, err2 := exec.Command("adb", "reverse", "tcp:8554", "tcp:8554").CombinedOutput()
	if err2 != nil {
		logMsg("ADB Lỗi Reverse RTSP (8554): %v (%s)", err2, strings.TrimSpace(string(out2)))
	} else {
		logMsg("ADB: Thiết lập Reverse RTSP (8554) thành công.")
	}
}

// ADB background polling
func startAdbWorker() {
	ticker := time.NewTicker(3 * time.Second)
	var wasConnected bool

	for range ticker.C {
		connected := checkDeviceConnected()
		if connected && !wasConnected {
			logMsg("ADB: Phát hiện điện thoại Android kết nối qua USB!")
			setupAdbReverseTunnels()
			wasConnected = true
		} else if !connected && wasConnected {
			logMsg("ADB: Thiết bị ngắt kết nối USB.")
			wasConnected = false
		}
	}
}

func checkDeviceConnected() bool {
	out, err := exec.Command("adb", "devices").Output()
	if err != nil {
		return false
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "device" {
			return true
		}
	}
	return false
}

// Fetch telemetry status for phone
func getAndroidStatus() AndroidStatus {
	connected := checkDeviceConnected()
	if !connected {
		return AndroidStatus{Connected: false}
	}

	// Model
	modelOut, _ := exec.Command("adb", "shell", "getprop", "ro.product.model").Output()
	model := strings.TrimSpace(string(modelOut))
	if model == "" {
		model = "Unknown Android"
	}

	// OS
	osOut, _ := exec.Command("adb", "shell", "getprop", "ro.build.version.release").Output()
	osVer := strings.TrimSpace(string(osOut))
	if osVer == "" {
		osVer = "N/A"
	}

	// Battery
	battery := BatteryInfo{Level: 0, Temperature: 0.0, Status: "N/A"}
	batOut, err := exec.Command("adb", "shell", "dumpsys", "battery").Output()
	if err == nil {
		lines := strings.Split(string(batOut), "\n")
		var isCharging bool
		var chargeSource string
		var statusVal int

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "AC powered: true") {
				isCharging = true
				chargeSource = "Nguồn sạc AC"
			} else if strings.HasPrefix(line, "USB powered: true") {
				isCharging = true
				chargeSource = "Cáp USB"
			} else if strings.HasPrefix(line, "level:") {
				fmt.Sscanf(line, "level: %d", &battery.Level)
			} else if strings.HasPrefix(line, "temp:") {
				var rawTemp int
				fmt.Sscanf(line, "temp: %d", &rawTemp)
				battery.Temperature = float64(rawTemp) / 10.0
			} else if strings.HasPrefix(line, "status:") {
				fmt.Sscanf(line, "status: %d", &statusVal)
			}
		}

		if isCharging {
			battery.Status = fmt.Sprintf("Sạc bằng %s", chargeSource)
		} else {
			switch statusVal {
			case 5:
				battery.Status = "Pin đầy"
			case 3:
				battery.Status = "Đang dùng pin (Xả)"
			case 4:
				battery.Status = "Không sạc"
			default:
				battery.Status = "Đang dùng pin"
			}
		}
	}

	return AndroidStatus{
		Connected: true,
		Model:     model,
		OSVersion: osVer,
		Battery:   battery,
	}
}

// Fetch telemetry status for TV Box host
func getHostStatus() HostStatus {
	cpuMu.Lock()
	cpuPercent := globalCpuPercent
	cpuMu.Unlock()

	return HostStatus{
		CPU:  cpuPercent,
		RAM:  getRamUsage(),
		Disk: getDiskUsage(),
	}
}

// Calculate CPU in background (2s intervals)
func startCpuWorker() {
	ticker := time.NewTicker(2 * time.Second)
	var prevIdle, prevTotal uint64

	for range ticker.C {
		idle, total, err := readCpuStats()
		if err != nil {
			continue
		}

		if prevTotal > 0 {
			diffIdle := idle - prevIdle
			diffTotal := total - prevTotal
			if diffTotal > 0 {
				percent := 100 - (diffIdle * 100 / diffTotal)
				cpuMu.Lock()
				globalCpuPercent = int(percent)
				cpuMu.Unlock()
			}
		}
		prevIdle = idle
		prevTotal = total
	}
}

func readCpuStats() (idle, total uint64, err error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, 0, fmt.Errorf("empty /proc/stat")
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("invalid cpu line")
	}

	var times [10]uint64
	for i := 1; i < len(fields); i++ {
		val, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		times[i-1] = val
		total += val
	}

	// Index 3 is idle time
	idle = times[3]
	return idle, total, nil
}

func getRamUsage() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	var total, available uint64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "MemTotal:" {
			total, _ = strconv.ParseUint(fields[1], 10, 64)
		} else if fields[0] == "MemAvailable:" {
			available, _ = strconv.ParseUint(fields[1], 10, 64)
		}
	}

	if total == 0 {
		return 0
	}

	used := total - available
	return int(used * 100 / total)
}

func getDiskUsage() int {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free

	if total == 0 {
		return 0
	}

	return int(used * 100 / total)
}

// Queries MediaMTX's internal paths API to check if a publisher is connected
func isStreamActive() bool {
	client := http.Client{Timeout: 500 * time.Millisecond}
	
	// Try standard v3 paths list endpoint first
	resp, err := client.Get("http://127.0.0.1:9997/v3/paths/list")
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		// Try fallback to legacy v3 paths
		resp, err = client.Get("http://127.0.0.1:9997/v3/paths")
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			// Try fallback to legacy v2 paths list
			resp, err = client.Get("http://127.0.0.1:9997/v2/paths/list")
			if err != nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					resp.Body.Close()
				}
				return false
			}
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Parse JSON loosely. If it contains "ready":true, a publisher is actively streaming
	return strings.Contains(string(body), `"ready":true`)
}
