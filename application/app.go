package application

import (
	"fmt"
	"github.com/VadimPushtaev/passive_balancer/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
)

type App struct {
	Context appContext
}

func NewApp() App {
	return App{Context: newAppContext()}
}

func (App App) RootHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		metrics.RPS.With(prometheus.Labels{"method": "GET"}).Inc()

		b := <-App.Context.Channel
		_, err := fmt.Fprintf(w, "%s\n", b)
		if err != nil {
			// TODO error metric
		}

	}
	if r.Method == http.MethodPost {
		metrics.RPS.With(prometheus.Labels{"method": "POST"}).Inc()

		b, _ := ioutil.ReadAll(r.Body)
		App.Context.Channel <- b
	}
}
