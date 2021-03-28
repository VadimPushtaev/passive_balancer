package main

import (
	"github.com/VadimPushtaev/passive_balancer/application"

	"github.com/pborman/getopt/v2"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	balancerApp := application.NewApp(getConfigPath())

	http.HandleFunc("/get", balancerApp.GetHandlerFunc)
	http.HandleFunc("/post", balancerApp.PostHandlerFunc)
	http.Handle("/metrics", promhttp.Handler())

	balancerApp.Run()
}

func getConfigPath() *string {
	var configPath string
	getopt.FlagLong(&configPath, "config", 'c', "path to config file")
	getopt.Parse()

	return &configPath
}
