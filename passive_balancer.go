package main

import (
	"github.com/VadimPushtaev/passive_balancer/application"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	balancerApp := application.NewApp()
	balancerApp.SetSignalHandlers()

	http.HandleFunc("/", balancerApp.RootHandlerFunc)
	http.Handle("/metrics", promhttp.Handler())

	balancerApp.Serve()
}
