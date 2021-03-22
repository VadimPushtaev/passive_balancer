package application

import (
	"github.com/stretchr/testify/assert"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestNewApp(t *testing.T) {
	NewApp()
}

func TestSetSignalHandlers(t *testing.T) {
	app := NewApp()
	defer signal.Reset()
	app.SetSignalHandlers()

	go func() {
		p, _ := os.FindProcess(syscall.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	select {
	case <-app.signalsChannel:
	case <-time.After(time.Second):
		t.Error("No signals in channel")
	}
}

func TestWaitSignal(t *testing.T) {
	app := NewApp()
	finished := make(chan bool)

	go func() {
		app.waitSignal()
		finished <- true
	}()

	app.signalsChannel <- syscall.SIGTERM
	app.signalsChannel <- syscall.SIGTERM

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Error("waitSignal is not finished")
	}

	assert.Equal(t, true, app.terminating)
	select {
	case <-app.doneChannel:
	case <-time.After(time.Second):
		t.Error("Server is not stopped")
	}
}

func TestFinishGracefully(t *testing.T) {
	app := NewApp()
	assert.Equal(
		t,
		0,
		app.finishGracefully(0, 0),
	)

	app.messageChannel <- []byte("")
	assert.Equal(
		t,
		7,
		app.finishGracefully(0, 7),
	)

}

func TestFinish(t *testing.T) {
	app := NewApp()
	app.finish()

	select {
	case <-app.doneChannel:
	case <-time.After(time.Second):
		t.Error("Server is not stopped")
	}
}
