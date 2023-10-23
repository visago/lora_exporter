package main

import (
	"strconv"
	"strings"
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
	Forward    string `env:"FORWARD" envDefault:""`
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

	if config.Debug {
		log.Logger = log.With().Caller().Logger()
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
			return file + ":" + strconv.Itoa(line)
		}
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Info().Int("interval", config.Interval).Str("buildVersion", BuildVersion).Str("buildTime", BuildTime).Str("buildBranch", BuildBranch).Str("buildRevision", BuildRevision).Msg("loraExporter started")

	initMetrics()
	startHttpServer()
	cron := gocron.NewScheduler(time.UTC)
	if len(config.Forward) > 0 {
		for _, url := range strings.Split(config.Forward, ",") {
			log.Info().Msgf("Will forward webhooks to %s", url)
		}
	}
	if len(config.ApiServer) > 0 {
		log.Info().Msgf("Will query %0s every %0ds", config.ApiServer, config.Interval)
		cron.Every(config.Interval).Seconds().SingletonMode().Do(getDeviceStatus)
	} else {
		log.Info().Msg("No APISERVER defined. Will not query Chirpstack for device status")
	}

	cron.StartBlocking()
}
