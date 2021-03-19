package main

import (
    "fmt"
    "io/ioutil"
    "net/http"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)


type appContext struct {
    Channel chan []byte
}


type application struct {
    Context appContext
}


func (App application) Root(w http.ResponseWriter, r *http.Request) {
    if (r.Method == http.MethodGet) {
        b := <- App.Context.Channel
        fmt.Fprintf(w, "%s\n", b)
    }
    if (r.Method == http.MethodPost) {
        b, _ := ioutil.ReadAll(r.Body)
        App.Context.Channel <- b
    }
}


func initMetrics() {
    promauto.NewCounter(prometheus.CounterOpts{
        Name: "passive_balancer_get_total",
        Help: "The total number of got messages",
    })
}


func main() {
    channel := make(chan []byte, 1024)
    context := appContext{Channel: channel}
    app := application{Context: context}

    http.HandleFunc("/", app.Root)
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":8090", nil)
}
