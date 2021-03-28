package application

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"reflect"
)

type AppConfiguration struct {
	Host                   string `yaml:"host" env:"PB_HOST" env-default:"localhost"`
	Port                   string `yaml:"port" env:"PB_PORT" env-default:"2308"`
	QueueSize              int    `yaml:"queue_size" env:"PB_QUEUE_SIZE" env-default:"1024"`
	GracefulPeriodSeconds  int    `yaml:"graceful_period_seconds" env:"PB_GRACEFUL_PERIOD_SECONDS" env-default:"60"`
	ShutdownTimeoutSeconds int    `yaml:"shutdown_timeout_seconds" env:"PB_SHUTDOWN_TIMEOUT_SECONDS" env-default:"3"`
	GetTimeoutSeconds      int    `yaml:"get_timeout_seconds" env:"PB_GET_TIMEOUT_SECONDS" env-default:"2"`
	PostTimeoutSeconds     int    `yaml:"post_timeout_seconds" env:"PB_POST_TIMEOUT_SECONDS" env-default:"2"`
}

func NewConfig(configPath *string) *AppConfiguration {
	var config AppConfiguration

	if configPath != nil && *configPath != "" {
		err := cleanenv.ReadConfig(*configPath, &config)
		if err != nil {
			panic(fmt.Sprintf("Couldn't intialize config from file `%s`: %s", *configPath, err))
		}
	} else {
		err := cleanenv.ReadEnv(&config)
		if err != nil {
			panic(fmt.Sprintf("Couldn't intialize config from env: %s", err))
		}
	}

	return &config
}

func (config *AppConfiguration) Print() {
	v := reflect.ValueOf(*config)

	values := make([]interface{}, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		values[i] = v.Field(i).Interface()
	}

	fmt.Println(values)
}
