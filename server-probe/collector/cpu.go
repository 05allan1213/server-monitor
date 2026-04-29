package collector

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v3/cpu"
)

type CPUCollector struct {
	hostname string
	usage    *prometheus.GaugeVec
}

func NewCPUCollector(hostname string) *CPUCollector {
	return &CPUCollector{
		hostname: hostname,
		usage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "server_monitor_cpu_usage_percent",
			Help: "Current CPU usage percentage.",
		}, []string{"instance"}),
	}
}

func (c *CPUCollector) Name() string {
	return "cpu"
}

func (c *CPUCollector) Register(registry *prometheus.Registry) {
	registry.MustRegister(c.usage)
}

func (c *CPUCollector) Update() error {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		return err
	}
	if len(percentages) == 0 {
		log.Printf("collector cpu: cpu.Percent returned empty slice")
		return nil
	}

	c.usage.WithLabelValues(c.hostname).Set(percentages[0])
	return nil
}
