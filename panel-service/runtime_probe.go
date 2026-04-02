package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type hostMetrics struct {
	CPUUsage      int
	CPUCores      int
	CPUModel      string
	RAMUsage      int
	RAMUsed       string
	RAMTotal      string
	DiskUsage     int
	DiskUsed      string
	DiskTotal     string
	UptimeSeconds int
	UptimeHuman   string
	LoadAvg       string
}

type securitySnapshot struct {
	FirewallActive         bool
	FirewallManager        string
	FirewallOpenPorts      []string
	ApacheBackendAvailable bool
	MailDomainAvailable    bool
	DetectedMailStack      []string
	DetectedWebStack       []string
	CloudLinuxAvailable    bool
	CloudLinuxEnabled      bool
	DetectedCloudLinux     []string
	ServerIP               string
	WireGuardActive        bool
	LivePatchingActive     bool
	EBPFMonitoring         bool
	MLWAFActive            bool
	DDoSGuardActive        bool
	OneClickHardening      bool
	SSHKeyManager          bool
	ImmutableOS            bool
}

type procStat struct {
	idle  uint64
	total uint64
}

func collectHostMetrics(startedAt time.Time) hostMetrics {
	cores := runtime.NumCPU()
	model := "Unknown CPU"
	if value := cpuModelName(); value != "" {
		model = value
	}

	cpuUsage := 0
	if value, ok := cpuUsagePercent(); ok {
		cpuUsage = value
	}

	ramUsedBytes, ramTotalBytes, ramUsage := memoryStats()
	diskUsedBytes, diskTotalBytes, diskUsage := diskStats()

	uptimeSeconds := int(time.Since(startedAt).Seconds())
	if value, ok := linuxUptimeSeconds(); ok {
		uptimeSeconds = value
	}

	uptimeHuman := (time.Duration(uptimeSeconds) * time.Second).Round(time.Second).String()
	loadAvg := "-"
	if value := linuxLoadAvg(); value != "" {
		loadAvg = value
	}

	return hostMetrics{
		CPUUsage:      cpuUsage,
		CPUCores:      cores,
		CPUModel:      model,
		RAMUsage:      ramUsage,
		RAMUsed:       humanBytesIEC(ramUsedBytes),
		RAMTotal:      humanBytesIEC(ramTotalBytes),
		DiskUsage:     diskUsage,
		DiskUsed:      humanBytesIEC(diskUsedBytes),
		DiskTotal:     humanBytesIEC(diskTotalBytes),
		UptimeSeconds: uptimeSeconds,
		UptimeHuman:   uptimeHuman,
		LoadAvg:       loadAvg,
	}
}

func collectSecuritySnapshot() securitySnapshot {
	firewallActive, firewallManager, firewallPorts := detectFirewallStatus()
	mailStack := detectMailStack()
	webStack := detectWebStack()
	cloudLinux := detectCloudLinuxStatus()
	cloudFeatures := []string{}
	for name, enabled := range cloudLinux.Features {
		if enabled {
			cloudFeatures = append(cloudFeatures, name)
		}
	}
	cloudFeatures = uniqueSortedStrings(cloudFeatures)

	return securitySnapshot{
		FirewallActive:         firewallActive,
		FirewallManager:        firewallManager,
		FirewallOpenPorts:      firewallPorts,
		ApacheBackendAvailable: apacheBackendAvailable(),
		MailDomainAvailable:    len(mailStack) >= 2,
		DetectedMailStack:      mailStack,
		DetectedWebStack:       webStack,
		CloudLinuxAvailable:    cloudLinux.Available,
		CloudLinuxEnabled:      cloudLinux.Enabled,
		DetectedCloudLinux:     cloudFeatures,
		ServerIP:               detectPrimaryIPv4(),
		WireGuardActive:        serviceActive("wg-quick@wg0", "wg-quick"),
		LivePatchingActive:     serviceActive("canonical-livepatch", "kpatch", "kgraft"),
		EBPFMonitoring:         serviceActive("cilium", "falco", "tetragon"),
		MLWAFActive:            detectModSecurityActive(),
		DDoSGuardActive:        loadDDoSRuntimeConfig().Enabled,
		OneClickHardening:      false,
		SSHKeyManager:          sshKeyManagerAvailable(),
		ImmutableOS:            false,
	}
}

func collectHostServices() []ServiceStatus {
	candidates := []struct {
		Name  string
		Desc  string
		Units []string
	}{
		{Name: "api-gateway", Desc: "AuraPanel API gateway", Units: []string{"aurapanel-api"}},
		{Name: "panel-service", Desc: "AuraPanel panel service", Units: []string{"aurapanel-service"}},
		{Name: "openlitespeed", Desc: "OpenLiteSpeed web server", Units: []string{"lshttpd", "openlitespeed", "lsws"}},
		{Name: "mariadb", Desc: "MariaDB database engine", Units: []string{"mariadb"}},
		{Name: "postgresql", Desc: "PostgreSQL database engine", Units: []string{"postgresql"}},
		{Name: "redis", Desc: "Redis in-memory data store", Units: []string{"redis-server", "redis"}},
		{Name: "docker", Desc: "Docker container runtime", Units: []string{"docker"}},
		{Name: "fail2ban", Desc: "Fail2Ban intrusion prevention", Units: []string{"fail2ban"}},
		{Name: "pdns", Desc: "PowerDNS authoritative daemon", Units: []string{"pdns"}},
		{Name: "postfix", Desc: "Postfix mail transport", Units: []string{"postfix"}},
		{Name: "dovecot", Desc: "Dovecot mail access", Units: []string{"dovecot"}},
		{Name: "pure-ftpd", Desc: "FTP service", Units: []string{"pure-ftpd"}},
		{Name: "minio", Desc: "MinIO object storage", Units: []string{"minio"}},
	}

	services := make([]ServiceStatus, 0, len(candidates))
	for _, candidate := range candidates {
		if status, ok := detectSystemdStatus(candidate.Units...); ok {
			services = append(services, ServiceStatus{
				Name:   candidate.Name,
				Desc:   candidate.Desc,
				Status: normalizeServiceState(status),
			})
		}
	}
	return services
}

func collectHostProcesses(limit int) []ProcessInfo {
	if limit <= 0 {
		limit = 15
	}
	if runtime.GOOS != "linux" {
		return []ProcessInfo{}
	}

	cmd := exec.Command("ps", "-eo", "pid,user,%cpu,%mem,comm", "--sort=-%cpu")
	output, err := cmd.Output()
	if err != nil {
		return []ProcessInfo{}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) <= 1 {
		return []ProcessInfo{}
	}

	processes := make([]ProcessInfo, 0, minInt(limit, len(lines)-1))
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		cpu, _ := strconv.ParseFloat(strings.ReplaceAll(fields[2], ",", "."), 64)
		mem, _ := strconv.ParseFloat(strings.ReplaceAll(fields[3], ",", "."), 64)
		command := strings.Join(fields[4:], " ")

		processes = append(processes, ProcessInfo{
			PID:     pid,
			User:    fields[1],
			CPU:     cpu,
			Mem:     mem,
			Command: command,
		})
		if len(processes) >= limit {
			break
		}
	}

	return processes
}

func executeServiceAction(serviceName, action string) error {
	unit, ok := serviceUnitName(serviceName)
	if !ok {
		return fmt.Errorf("service not supported")
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("service control is only available on linux hosts")
	}
	cmd := exec.Command("systemctl", action, unit)
	return cmd.Run()
}

func terminateProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("process control is only available on linux hosts")
	}
	cmd := exec.Command("kill", "-TERM", strconv.Itoa(pid))
	return cmd.Run()
}

func serviceUnitName(serviceName string) (string, bool) {
	switch strings.TrimSpace(serviceName) {
	case "api-gateway":
		return "aurapanel-api.service", true
	case "panel-service":
		return "aurapanel-service.service", true
	case "openlitespeed":
		return "lshttpd.service", true
	case "mariadb":
		return "mariadb.service", true
	case "postgresql":
		return "postgresql.service", true
	case "redis":
		if systemctlUnitExists("redis-server.service") {
			return "redis-server.service", true
		}
		return "redis.service", true
	case "docker":
		return "docker.service", true
	case "fail2ban":
		return "fail2ban.service", true
	case "pdns":
		return "pdns.service", true
	case "postfix":
		return "postfix.service", true
	case "dovecot":
		return "dovecot.service", true
	case "pure-ftpd":
		return "pure-ftpd.service", true
	case "minio":
		return "minio.service", true
	default:
		return "", false
	}
}

func systemctlUnitExists(unit string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	cmd := exec.Command("systemctl", "list-unit-files", unit)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), unit)
}

func detectSystemdStatus(units ...string) (string, bool) {
	if runtime.GOOS != "linux" {
		return "", false
	}
	for _, unit := range units {
		cmd := exec.Command("systemctl", "show", "--property=LoadState,ActiveState", unit)
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		loadState := ""
		activeState := ""
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "LoadState="):
				loadState = strings.TrimPrefix(line, "LoadState=")
			case strings.HasPrefix(line, "ActiveState="):
				activeState = strings.TrimPrefix(line, "ActiveState=")
			}
		}

		if loadState == "" || loadState == "not-found" {
			continue
		}
		return activeState, true
	}
	return "", false
}

func normalizeServiceState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active":
		return "running"
	case "activating":
		return "starting"
	case "reloading":
		return "starting"
	case "failed":
		return "failed"
	default:
		return "stopped"
	}
}

func serviceActive(units ...string) bool {
	status, ok := detectSystemdStatus(units...)
	return ok && strings.EqualFold(status, "active")
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func apacheBackendAvailable() bool {
	return commandExists("apache2") || commandExists("httpd") || serviceActive("apache2", "httpd")
}

func detectMailStack() []string {
	stack := []string{}
	if serviceActive("postfix") {
		stack = append(stack, "postfix")
	}
	if serviceActive("dovecot") {
		stack = append(stack, "dovecot")
	}
	sort.Strings(stack)
	return stack
}

func detectWebStack() []string {
	stack := []string{}
	if serviceActive("lshttpd", "openlitespeed", "lsws") {
		stack = append(stack, "openlitespeed")
	}
	if apacheBackendAvailable() {
		stack = append(stack, "apache")
	}
	sort.Strings(stack)
	return stack
}

func detectFirewallStatus() (bool, string, []string) {
	if runtime.GOOS != "linux" {
		return false, "", nil
	}

	if commandExists("ufw") {
		cmd := exec.Command("ufw", "status")
		output, err := cmd.Output()
		if err == nil {
			text := string(output)
			active := strings.Contains(strings.ToLower(text), "status: active")
			return active, "ufw", parseUFWOpenPorts(text)
		}
	}

	if commandExists("nft") {
		cmd := exec.Command("nft", "list", "ruleset")
		output, err := cmd.Output()
		if err == nil {
			text := strings.TrimSpace(string(output))
			if text != "" {
				return true, "nftables", parseNftOpenPorts(text)
			}
		}
	}

	return false, "", nil
}

func parseUFWOpenPorts(raw string) []string {
	ports := make(map[string]struct{})
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Status:") || strings.HasPrefix(line, "To") || strings.HasPrefix(line, "--") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		ports[fields[0]] = struct{}{}
	}
	return sortedKeys(ports)
}

func parseNftOpenPorts(raw string) []string {
	ports := make(map[string]struct{})
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, " dport ") {
			for _, field := range strings.Fields(line) {
				if strings.Contains(field, "/") || strings.Contains(field, ":") {
					continue
				}
				if strings.HasPrefix(field, "dport") || strings.HasPrefix(field, "sport") {
					continue
				}
				if strings.ContainsAny(field, "0123456789") {
					ports[field] = struct{}{}
				}
			}
		}
	}
	return sortedKeys(ports)
}

func sortedKeys(items map[string]struct{}) []string {
	values := make([]string, 0, len(items))
	for item := range items {
		values = append(values, item)
	}
	sort.Strings(values)
	return values
}

func detectPrimaryIPv4() string {
	if value := strings.TrimSpace(os.Getenv("AURAPANEL_PUBLIC_IP")); value != "" {
		return value
	}

	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil {
			if ip := addr.IP.To4(); ip != nil {
				return ip.String()
			}
		}
	}

	return "127.0.0.1"
}

func cpuUsagePercent() (int, bool) {
	if runtime.GOOS != "linux" {
		return 0, false
	}

	first, err := readProcStat()
	if err != nil {
		return 0, false
	}
	time.Sleep(150 * time.Millisecond)
	second, err := readProcStat()
	if err != nil {
		return 0, false
	}

	totalDelta := second.total - first.total
	idleDelta := second.idle - first.idle
	if totalDelta == 0 {
		return 0, false
	}

	usedDelta := totalDelta - idleDelta
	return int((float64(usedDelta) / float64(totalDelta)) * 100), true
}

func readProcStat() (procStat, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return procStat{}, err
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return procStat{}, fmt.Errorf("invalid /proc/stat format")
	}

	var total uint64
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return procStat{}, err
		}
		values = append(values, value)
		total += value
	}

	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}

	return procStat{idle: idle, total: total}, nil
}

func cpuModelName() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.ToLower(line), "model name") {
			if _, value, ok := strings.Cut(line, ":"); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func memoryStats() (usedBytes uint64, totalBytes uint64, usagePercent int) {
	if runtime.GOOS != "linux" {
		return 0, 0, 0
	}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}

	values := map[string]uint64{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}

	totalBytes = values["MemTotal"]
	availableBytes := values["MemAvailable"]
	if totalBytes == 0 {
		return 0, 0, 0
	}

	if availableBytes > totalBytes {
		availableBytes = 0
	}
	usedBytes = totalBytes - availableBytes
	usagePercent = int(float64(usedBytes) / float64(totalBytes) * 100)
	return usedBytes, totalBytes, usagePercent
}

func diskStats() (usedBytes uint64, totalBytes uint64, usagePercent int) {
	if runtime.GOOS != "linux" {
		return 0, 0, 0
	}

	cmd := exec.Command("df", "-B1", "/")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return 0, 0, 0
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 5 {
		return 0, 0, 0
	}

	totalBytes, _ = strconv.ParseUint(fields[1], 10, 64)
	usedBytes, _ = strconv.ParseUint(fields[2], 10, 64)
	if totalBytes > 0 {
		usagePercent = int(float64(usedBytes) / float64(totalBytes) * 100)
	}
	return usedBytes, totalBytes, usagePercent
}

func linuxUptimeSeconds() (int, bool) {
	if runtime.GOOS != "linux" {
		return 0, false
	}
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	return int(value), true
}

func linuxLoadAvg() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return ""
	}
	return strings.Join(fields[:3], " ")
}

func humanBytesIEC(value uint64) string {
	if value == 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}

	if unit == 0 {
		return fmt.Sprintf("%d %s", value, units[unit])
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}
