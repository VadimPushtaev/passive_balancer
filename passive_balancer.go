package main

import (
	"fmt"
	"github.com/VadimPushtaev/passive_balancer/application"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	balancerApp := application.NewApp()

	http.HandleFunc("/", balancerApp.RootHandlerFunc)
	http.Handle("/metrics", promhttp.Handler())

	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		fmt.Printf("Failed to listen and serve")
	}
}
