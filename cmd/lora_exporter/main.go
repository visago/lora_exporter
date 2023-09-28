package main

import (
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	BuildBranch   string
	BuildVersion  string
	BuildTime     string
	BuildRevision string
)

type EnvConfig struct {
	Interval   int    `env:"INTERVAL,required" envDefault:"300"`
	DumpFolder string `env:"DUMP_FOLDER,required" envDefault:"/tmp/lora"`
	Listen     string `env:"LISTEN,required" envDefault:"0.0.0.0:5672"`
	Debug      bool   `env:"DEBUG" envDefault:false`
	ApiFile    string `env:"APIFILE" envDefault:"apikey.txt"`
	ApiKey     string `env:"APIKEY"`
	ApiServer  string `env:"APISERVER"`
	AuthKey    string `env:"AUTHKEY"`
}

var config EnvConfig

func main() {
	if err := env.Parse(&config); err != nil {
		log.Error().Err(err).Msg("Failed to parse env configuration")
	}
	log.Info().Int("interval", config.Interval).Str("buildVersion", BuildVersion).Str("buildTime", BuildTime).Str("buildBranch", BuildBranch).Str("buildRevision", BuildRevision).Msg("loraExporter started")

	if config.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Debug().Msg("debug logging enabled")
	} else {

		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	initMetrics()
	startHttpServer()
	cron := gocron.NewScheduler(time.UTC)
	if len(config.ApiServer) > 0 {
		log.Info().Msgf("Will query %0s every %0ds", config.ApiServer, config.Interval)
		cron.Every(config.Interval).Seconds().SingletonMode().Do(getDeviceStatus)
	} else {
		log.Info().Msg("No APISERVER defined. Will not query Chirpstack for device status")
	}

	cron.StartBlocking()
}
