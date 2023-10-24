package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

func startForwardServer() {
	go func() {
		for {
			body := <-backgroundChannel
			log.Debug().Int("size", len(body)).Msg("Got background webhook forward request")
			if len(config.Forward) > 0 {
				for _, url := range strings.Split(config.Forward, ",") {
					log.Debug().Str("url", url).Msg("forwarding")
					req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
					if err != nil {
						forwardConnectionErrorTotal.With(prometheus.Labels{"url": url}).Inc()
						log.Error().Err(err).Str("url", url).Msg("Failed to create outgoing forward request")
						break
					}
					req.Header.Set("Content-Type", "application/json")
					res, err2 := http.DefaultClient.Do(req)
					if err2 != nil {
						log.Error().Err(err2).Str("url", url).Msg("Failed to do outgoing forward request")
						break
					}
					if res.StatusCode == 200 {
						log.Debug().Int("status", res.StatusCode).Str("url", url).Msg("Forwarded webhook")
						forwardConnectionSuccessTotal.With(prometheus.Labels{"url": url}).Inc()
					} else {
						log.Error().Int("status", res.StatusCode).Str("url", url).Msg("Forwarded webhook but got non-200 reply")
						forwardConnectionErrorTotal.With(prometheus.Labels{"url": url}).Inc()

					}
				}
			}
		}
	}()
}

func startHttpServer() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", webhookHandler)
	mux.HandleFunc("/hook", webhookHandler)
	mux.HandleFunc("/dump", dumpHandler)
	httpServer := &http.Server{
		Addr:         config.Listen,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	log.Info().Msgf("Listening on port %s for http requests", config.Listen)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				log.Fatal().Err(err).Msg("Failed to start http server")
			}
		}
	}()
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	ua := filterAscii(r.Header.Get("User-Agent"))
	auth := filterAscii(r.Header.Get("Authorization"))
	ip := ReadUserIP(r)
	switch r.Method {
	case "POST":
		webhookConnectionTotal.With(prometheus.Labels{"ip": ip}).Inc()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error().Err(err).Str("IP", ip).Str("User-Agent", ua).Str("Authorization", auth).Msg("Failed to read request body")
			webhookConnectionErrorTotal.With(prometheus.Labels{"ip": ip}).Inc()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		filename := ""
		devEui, needDump, err2 := parseChirpstackWebhook(body)
		if config.Debug || needDump {
			filename = dumpFile(body)
		}
		if err2 != nil {
			webhookConnectionErrorTotal.With(prometheus.Labels{"ip": ip}).Inc()
			if filename == "" { // We already dumped it since its in debug mode
				filename = dumpFile(body)
			}
			log.Error().Err(err2).Str("dump", filename).Str("IP", ip).Str("User-Agent", ua).Str("Authorization", auth).Msg("Failed to parse request body")
			http.Error(w, err2.Error(), http.StatusBadRequest)
			return
		}
		if len(config.Forward) > 0 {
			log.Debug().Int("size", len(body)).Msg("Forward webhook body to background task")
			backgroundChannel <- body
		}
		fmt.Fprintf(w, `ok`)
		log.Info().Str("devEui", devEui).Str("dump", filename).Str("method", r.Method).Str("IP", ip).Str("User-Agent", ua).Str("Authorization", auth).Int("size", len(body)).Msg("Got webhook request")
	default:
		log.Info().Str("method", r.Method).Str("IP", ip).Str("User-Agent", ua).Str("Authorization", auth).Msg("Got lost soul")
		http.Redirect(w, r, "/metrics", http.StatusMovedPermanently)
	}
}

func dumpHandler(w http.ResponseWriter, r *http.Request) {
	ip := ReadUserIP(r)
	webhookConnectionTotal.With(prometheus.Labels{"ip": ip}).Inc()
	ua := filterAscii(r.Header.Get("User-Agent"))
	auth := filterAscii(r.Header.Get("Authorization"))
	log.Debug().Str("User-Agent", ua).Str("Authorization", auth).Msg("Got request")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to ready request body")
		webhookConnectionErrorTotal.With(prometheus.Labels{"ip": ip}).Inc()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	filename := dumpFile(body)
	fmt.Fprintf(w, "dumped to %s\n", filename)

}

func dumpFile(s []byte) string {
	filename := config.DumpFolder + time.Now().Format("/20060102-150405.000.dump")
	f, err := os.Create(filename)
	if err != nil {
		log.Error().Err(err).Str("filename", filename).Msg("Failed to open file to write")
	}
	defer f.Close()
	nb, err2 := f.Write(s)
	if err2 != nil {
		log.Error().Err(err).Str("filename", filename).Msg("Failed to write stringo to file")
	} else {
		log.Debug().Str("filename", filename).Msgf("Wrote %0d bytes to file", nb)
	}
	return filename
}

func filterAscii(s string) string {
	return strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII {
			return -1
		}
		return r
	}, s)
}

func PrettyPrint(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		fmt.Println(string(b))
	}
}

func ReadUserIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	if strings.ContainsRune(IPAddress, ':') {
		IPAddress, _, _ = net.SplitHostPort(IPAddress)
	}
	return IPAddress
}
