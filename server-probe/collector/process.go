package collector

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/process"
)

type ProcessCollector struct {
	hostname     string
	processCount *prometheus.GaugeVec
	uptime       *prometheus.GaugeVec
}

func NewProcessCollector(hostname string) *ProcessCollector {
	return &ProcessCollector{
		hostname: hostname,
		processCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "server_monitor_process_count",
			Help: "Current number of processes.",
		}, []string{"instance"}),
		uptime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "server_monitor_uptime_seconds",
			Help: "System uptime in seconds.",
		}, []string{"instance"}),
	}
}

func (c *ProcessCollector) Name() string {
	return "process"
}

func (c *ProcessCollector) Register(registry *prometheus.Registry) {
	registry.MustRegister(c.processCount, c.uptime)
}

func (c *ProcessCollector) Update() error {
	pids, err := process.Pids()
	if err != nil {
		return err
	}
	c.processCount.WithLabelValues(c.hostname).Set(float64(len(pids)))

	bootTime, err := host.BootTime()
	if err != nil {
		return err
	}
	uptime := time.Since(time.Unix(int64(bootTime), 0)).Seconds()
	if uptime < 0 {
		uptime = 0
	}
	c.uptime.WithLabelValues(c.hostname).Set(uptime)

	return nil
}
