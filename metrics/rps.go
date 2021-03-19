package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var RPS = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "passive_balancer_rps_total",
		Help: "The total number of requests per second",
	},
	[]string{"method"},
)
