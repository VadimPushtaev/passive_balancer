package application

import (
	"context"
	"fmt"
	"github.com/VadimPushtaev/passive_balancer/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type App struct {
	messageChannel chan []byte
	server         *http.Server
	signalsChannel chan os.Signal
	doneChannel    chan bool
	terminating    bool
	finishOnce     *sync.Once
}

func NewApp() App {
	server := http.Server{Addr: ":8090"}

	return App{
		messageChannel: make(chan []byte, 1024),
		server:         &server,
		signalsChannel: make(chan os.Signal),
		doneChannel:    make(chan bool),
		terminating:    false,
		finishOnce:     &sync.Once{},
	}
}

func (app App) SetSignalHandlers() {
	signal.Notify(app.signalsChannel, syscall.SIGINT, syscall.SIGTERM)
	go app.waitSignal()
}

func (app App) waitSignal() {
	for {
		<-app.signalsChannel
		if app.terminating {
			// Signal repeated, shutting down immediately
			app.finishOnce.Do(app.finish)
			break
		} else {
			app.terminating = true
			go app.finishGracefully()
		}
	}
}

func (app App) finishGracefully() {
	for i := 10; i > 0; i-- {
		messagesLeft := len(app.messageChannel)
		if messagesLeft == 0 {
			break
		}
		fmt.Printf("Terminating: %d more tries, %d more messages\n", i, messagesLeft)
		time.Sleep(time.Second)
	}

	app.finishOnce.Do(app.finish)
}

func (app App) finish() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer func() {
		cancel()
	}()

	err := app.server.Shutdown(ctx)
	if err != nil {
		fmt.Printf("Failed to shut down: %s\n", err)
	}

	app.doneChannel <- true
}

func (app App) Serve() {
	err := app.server.ListenAndServe()
	if err == http.ErrServerClosed {
		<-app.doneChannel
	} else {
		fmt.Printf("Failed to listen and serve: %s\n", err)
	}
}

func (app App) RootHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		metrics.RPS.With(prometheus.Labels{"method": "GET"}).Inc()

		select {
		case b := <-app.messageChannel:
			_, err := fmt.Fprintf(w, "%s\n", b)
			if err != nil {
				// TODO error metric
			}
		case <-time.After(2 * time.Second):
			http.Error(w, "Timout exceeded", http.StatusInternalServerError)
		}
	}
	if r.Method == http.MethodPost {
		metrics.RPS.With(prometheus.Labels{"method": "POST"}).Inc()

		if app.terminating {
			http.Error(w, "Service is terminating", http.StatusInternalServerError)
		} else {
			b, _ := ioutil.ReadAll(r.Body)
			r.Body.Close()

			select {
			case app.messageChannel <- b:
			case <-time.After(2 * time.Second):
				http.Error(w, "Timout exceeded", http.StatusInternalServerError)
			}
		}
	}
}
