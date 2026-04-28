package collector

import "github.com/prometheus/client_golang/prometheus"

type Collector interface {
	Name() string
	Register(registry *prometheus.Registry)
	Update() error
}
