package promclient

const (
	queryHostUp      = `up{job="server-probe"}`
	queryCPUUsage    = "server_monitor_cpu_usage_percent"
	queryMemoryUsage = "server_monitor_memory_usage_percent"
)
