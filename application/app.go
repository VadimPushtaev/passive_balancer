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
	config         *AppConfiguration
	messageChannel chan []byte
	server         *http.Server
	signalsChannel chan os.Signal
	doneChannel    chan bool
	terminating    bool
	finishOnce     *sync.Once
}

func NewApp(configPath *string) *App {
	config := NewConfig(configPath)
	fmt.Printf("%+v\n", *config)
	server := http.Server{Addr: fmt.Sprintf("%s:%s", config.Host, config.Port)}

	return &App{
		config:         config,
		messageChannel: make(chan []byte, config.QueueSize),
		server:         &server,
		signalsChannel: make(chan os.Signal),
		doneChannel:    make(chan bool, 1),
		terminating:    false,
		finishOnce:     &sync.Once{},
	}
}

func NewAppWithoutConfigFile() *App {
	return NewApp(nil)
}

func (app *App) Run() {
	app.SetSignalHandlers()
	go app.waitSignal()
	app.Serve()
}

func (app *App) SetSignalHandlers() {
	signal.Notify(app.signalsChannel, syscall.SIGINT, syscall.SIGTERM)
}

func (app *App) waitSignal() {
	for {
		<-app.signalsChannel
		if app.terminating {
			// Signal repeated, shutting down immediately
			app.finishOnce.Do(app.finish)
			break
		} else {
			app.terminating = true
			go app.finishGracefully(time.Second, app.config.GracefulPeriodSeconds)
		}
	}
}

// Returns the number of iterations (starting with 0)
func (app *App) finishGracefully(timeout time.Duration, limit int) int {
	i := 0
	for ; i < limit || limit == 0; i++ {
		messagesLeft := len(app.messageChannel)
		if messagesLeft == 0 {
			break
		}
		if limit > 0 {
			fmt.Printf("Terminating: %d more tries, %d more messages\n", limit-i-1, messagesLeft)
		} else {
			fmt.Printf("Terminating: %d more messages\n", messagesLeft)
		}
		time.Sleep(timeout)
	}

	app.finishOnce.Do(app.finish)

	return i
}

func (app *App) finish() {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(app.config.ShutdownTimeoutSeconds)*time.Second,
	)
	defer func() {
		cancel()
	}()

	err := app.server.Shutdown(ctx)
	if err != nil {
		fmt.Printf("Failed to shut down: %s\n", err)
	}

	app.doneChannel <- true
}

func (app *App) Serve() {
	err := app.server.ListenAndServe()
	if err == http.ErrServerClosed {
		<-app.doneChannel
	} else {
		fmt.Printf("Failed to listen and serve: %s\n", err)
	}
}

func (app *App) GetHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}

	metrics.RPS.With(prometheus.Labels{"method": "GET"}).Inc()

	select {
	case b := <-app.messageChannel:
		_, err := fmt.Fprintf(w, "%s\n", b)
		if err != nil {
			// TODO error metric
		}
	case <-time.After(time.Duration(app.config.GetTimeoutSeconds) * time.Second):
		http.Error(w, "Timeout exceeded", http.StatusInternalServerError)
	}
}
func (app *App) PostHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}

	metrics.RPS.With(prometheus.Labels{"method": "POST"}).Inc()

	if app.terminating {
		http.Error(w, "Service is terminating", http.StatusInternalServerError)
	} else {
		defer r.Body.Close()
		b, _ := ioutil.ReadAll(r.Body)

		select {
		case app.messageChannel <- b:
		case <-time.After(time.Duration(app.config.PostTimeoutSeconds) * time.Second):
			http.Error(w, "Timeout exceeded", http.StatusInternalServerError)
		}
	}
}
