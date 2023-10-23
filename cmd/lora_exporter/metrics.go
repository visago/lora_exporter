package main

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricsPrefix = "lora"
)

var (
	labelsMap = map[string]prometheus.Labels{}

	labelsDeviceGateway  = []string{"gatewayId", "deviceName", "deviceEui"}
	labelsDevice         = []string{"deviceName", "deviceEui"}
	labelsDeviceMsgLevel = []string{"deviceName", "deviceEui", "level", "code"}
	labelsDeviceMetric   = []string{"deviceName", "deviceEui", "type"}
	labelsForward        = []string{"url"}

	webhookConnectionTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: metricsPrefix + "_webhook_total",
		Help: "The total number of connections",
	})
	webhookConnectionErrorTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: metricsPrefix + "_webhook_error_total",
		Help: "The total number of errors",
	})

	forwardConnectionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "_forward_total",
		Help: "The total number of forwarded webhooks",
	}, labelsForward)
	forwardConnectionErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "_forward_error_total",
		Help: "The total number of forwarded webhooks errors",
	}, labelsForward)

	grpcConnectionTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: metricsPrefix + "_grpc_connection_total",
		Help: "The total number of connections",
	})
	grpcConnectionErrorTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: metricsPrefix + "_grpc_connection_error_total",
		Help: "The total number of errors",
	})
	grpcApiTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: metricsPrefix + "_grpc_api_total",
		Help: "The total number of grpc api calls",
	})
	grpcApiErrorTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: metricsPrefix + "_grpc_api_error_total",
		Help: "The total number of errors for grpc api calls",
	})

	deviceFcnt = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricsPrefix + "_devices_fcnt",
		Help: "Frame Count of device",
	}, labelsDevice,
	)

	deviceUnconfirmed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "_devices_unconfirmed_count",
		Help: "unconfirmed count",
	}, labelsDevice,
	)
	deviceConfirmed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "_devices_confirmed_count",
		Help: "confirmed count",
	}, labelsDevice,
	)
	deviceMsgLevelCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "_devices_msg_level_count",
		Help: "device msg level/type count",
	}, labelsDeviceMsgLevel,
	)
	deviceBattery = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricsPrefix + "_devices_battery_percent",
		Help: "Battery level of device",
	}, labelsDevice,
	)
	deviceExternalPower = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricsPrefix + "_devices_externalpower",
		Help: "External powersource of device",
	}, labelsDevice,
	)

	deviceMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricsPrefix + "_devices_metric",
		Help: "metric value of device",
	}, labelsDeviceMetric,
	)

	deviceLastseen = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricsPrefix + "_devices_lastseen",
		Help: "last seen value of device",
	}, labelsDeviceGateway,
	)
	deviceRxInfoRssi = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricsPrefix + "_devices_rxinfo_rssi_db",
		Help: "RSSI of RX from device",
	}, labelsDeviceGateway,
	)
	deviceRxInfoSnr = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricsPrefix + "_devices_rxinfo_snr_db",
		Help: "SNR of RX from device",
	}, labelsDeviceGateway,
	)

	buildInfo = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "build_info",
			Help: "A metric with a constant '1' value labeled by goversion used to build the binary.",
			ConstLabels: map[string]string{
				"appname":       "loraExporter",
				"buildVersion":  BuildVersion,
				"buildTime":     BuildTime,
				"buildBranch":   BuildBranch,
				"buildRevision": BuildRevision,

				"goversion": runtime.Version(),
			},
		},
	)
)

func initMetrics() {
	buildInfo.Set(1)
	labelsMap = make(map[string]prometheus.Labels)
}
