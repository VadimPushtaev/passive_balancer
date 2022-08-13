package application

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/VadimPushtaev/passive_balancer/metrics"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type CallbackMessage struct {
	bytes []byte
}

type Message struct {
	bytes           []byte
	callbackEnabled bool
	callbackChannel chan CallbackMessage
}

type App struct {
	config         *AppConfiguration
	logger         *zap.Logger
	messageChannel chan Message
	server         *http.Server
	signalsChannel chan os.Signal
	doneChannel    chan bool
	terminating    bool
	finishOnce     *sync.Once
}

func NewApp(configPath *string) *App {
	logger, _ := zap.NewProduction()

	config := NewConfig(configPath)
	logger.Sugar().Infow(
		"Configuration is set",
		"config", config,
	)
	server := http.Server{Addr: fmt.Sprintf("%s:%s", config.Host, config.Port)}

	return &App{
		config:         config,
		logger:         logger,
		messageChannel: make(chan Message, config.QueueSize),
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
	app.logger.Sugar().Info("Run is started")

	app.SetSignalHandlers()
	go app.waitSignal()
	app.Serve()

	app.logger.Sugar().Info("Run is finished")
}

func (app *App) SetSignalHandlers() {
	signal.Notify(app.signalsChannel, syscall.SIGINT, syscall.SIGTERM)
}

func (app *App) waitSignal() {
	for {
		receivedSignal := <-app.signalsChannel
		if app.terminating {
			app.logger.Sugar().Warnw(
				"Signal received again, shutting down immediately",
				"signal", receivedSignal,
			)
			app.finishOnce.Do(app.finish)
			break
		} else {
			app.logger.Sugar().Warnw(
				"Signal received, terminating now",
				"signal", receivedSignal,
			)
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
			app.logger.Sugar().Warnf(
				"Terminating: %d more tries, %d more messages\n",
				limit-i-1, messagesLeft,
			)
		} else {
			app.logger.Sugar().Warnf("Terminating: %d more messages\n", messagesLeft)
		}
		time.Sleep(timeout)
	}

	app.finishOnce.Do(app.finish)

	return i
}

func (app *App) finish() {
	app.logger.Sugar().Warn("Shutting down now")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(app.config.ShutdownTimeoutSeconds)*time.Second,
	)
	defer func() {
		cancel()
	}()

	err := app.server.Shutdown(ctx)
	if err != nil {
		app.logger.Sugar().Errorf("Failed to shut down: %s\n", err)
	}

	app.doneChannel <- true
}

func (app *App) Serve() {
	err := app.server.ListenAndServe()
	if err == http.ErrServerClosed {
		<-app.doneChannel
	} else {
		app.logger.Sugar().Errorf("Failed to listen and serve: %s\n", err)
	}
}

func (app *App) GetHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}

	metrics.RequestsTotal.With(prometheus.Labels{"location": "get"}).Inc()

	timedOut := false
	select {
	case m := <-app.messageChannel:
		if m.callbackEnabled {
			bodies := r.URL.Query()["body"]
			body := ""
			if len(bodies) > 0 {
				body = bodies[len(bodies)-1]
			}
			m.callbackChannel <- CallbackMessage{[]byte(body)}
		}
		_, err := fmt.Fprintf(w, "%s\n", m.bytes)
		if err != nil {
			app.logger.Warn("Failed to send data to a client")
		}
	case <-time.After(time.Duration(app.config.GetTimeoutSeconds) * time.Second):
		timedOut = true
		http.Error(w, "Timeout exceeded", http.StatusInternalServerError)
		metrics.TimeoutsTotal.With(prometheus.Labels{"location": "get"}).Inc()
	}

	app.logger.Info(
		"HTTP request is served",
		zap.String("request", "get"),
		zap.Bool("timed_out", timedOut),
	)
}
func (app *App) PostHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}

	metrics.RequestsTotal.With(prometheus.Labels{"location": "post"}).Inc()

	denied := false
	timedOut := false
	if app.terminating {
		denied = true
		http.Error(w, "Service is terminating", http.StatusInternalServerError)
	} else {
		defer r.Body.Close()
		b, _ := ioutil.ReadAll(r.Body)

		message := Message{
			b,
			false,
			nil,
		}

		select {
		case app.messageChannel <- message:
		case <-time.After(time.Duration(app.config.PostTimeoutSeconds) * time.Second):
			timedOut = true
			http.Error(w, "Timeout exceeded", http.StatusInternalServerError)
		}
	}

	app.logger.Info(
		"HTTP request is served",
		zap.String("request", "post"),
		zap.Bool("timedOut", timedOut),
		zap.Bool("denied", denied),
	)
}

func (app *App) PostWithCallbackHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}

	metrics.RequestsTotal.With(prometheus.Labels{"location": "post_with_callback"}).Inc()

	denied := false
	timedOut := false
	if app.terminating {
		denied = true
		http.Error(w, "Service is terminating", http.StatusInternalServerError)
	} else {
		defer r.Body.Close()
		b, _ := ioutil.ReadAll(r.Body)

		message := Message{
			b,
			true,
			make(chan CallbackMessage, 1),
		}

		select {
		case app.messageChannel <- message:
		case <-time.After(time.Duration(app.config.PostTimeoutSeconds) * time.Second):
			timedOut = true
			http.Error(w, "Timeout exceeded", http.StatusInternalServerError)
		}

		callbackMessage := <-message.callbackChannel
		_, err := fmt.Fprintf(w, "%s\n", callbackMessage.bytes)
		if err != nil {
			app.logger.Warn("Failed to send data to a client")
		}
	}

	app.logger.Info(
		"HTTP request is served",
		zap.String("request", "post"),
		zap.Bool("timedOut", timedOut),
		zap.Bool("denied", denied),
	)
}
