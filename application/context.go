package application

type appContext struct {
	Channel chan []byte
}

func newAppContext() appContext {
	return appContext{Channel: make(chan []byte, 1024)}
}
