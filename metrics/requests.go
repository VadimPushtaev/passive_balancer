package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var RequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "passive_balancer_requests_total",
		Help: "The total number of requests",
	},
	[]string{"location"},
)

var TimeoutsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "passive_balancer_timeouts_total",
		Help: "The total number of timeouts",
	},
	[]string{"location"},
)
