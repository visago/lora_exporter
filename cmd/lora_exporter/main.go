package main

import (
	"runtime"
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
	DumpFolder string `env:"DUMP_FOLDER" envDefault:""`
	Listen     string `env:"LISTEN,required" envDefault:"0.0.0.0:5672"`
	Forward    string `env:"FORWARD" envDefault:""`
	Debug      bool   `env:"DEBUG" envDefault:false`
	ApiFile    string `env:"APIFILE" envDefault:"apikey.txt"`
	ApiKey     string `env:"APIKEY"`
	ApiServer  string `env:"APISERVER"`
	AuthKey    string `env:"AUTHKEY"`
	MetricsGeo bool   `env:"METRICS_GEO" envDefault:false`
}

var config EnvConfig
var backgroundChannel chan []byte

func main() {
	if err := env.Parse(&config); err != nil {
		log.Error().Err(err).Msg("Failed to parse env configuration")
	}
	cron := gocron.NewScheduler(time.UTC)

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
		cron.Every(config.Interval).Minutes().SingletonMode().Do(printMemUsage)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Info().Int("interval", config.Interval).Str("buildVersion", BuildVersion).Str("buildTime", BuildTime).Str("buildBranch", BuildBranch).Str("buildRevision", BuildRevision).Msg("loraExporter started")

	initMetrics()
	if len(config.Forward) > 0 {
		backgroundChannel = make(chan []byte)
		startForwardServer()
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
	startHttpServer()
	cron.StartBlocking()
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Info().Uint64("alloc", m.Alloc).Uint64("totalAlloc", m.TotalAlloc).Uint64("sys", m.Sys).Uint32("numGC", m.NumGC).Msg("memory usage dump")
}
