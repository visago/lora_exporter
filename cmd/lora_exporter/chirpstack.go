package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chirpstack/chirpstack/api/go/v4/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type WebHookDoc struct {
	DeduplicationID string    `json:"deduplicationId"`
	Time            time.Time `json:"time"`
	DeviceInfo      struct {
		TenantID           string `json:"tenantId"`
		TenantName         string `json:"tenantName"`
		ApplicationID      string `json:"applicationId"`
		ApplicationName    string `json:"applicationName"`
		DeviceProfileID    string `json:"deviceProfileId"`
		DeviceProfileName  string `json:"deviceProfileName"`
		DeviceName         string `json:"deviceName"`
		DevEui             string `json:"devEui"`
		DeviceClassEnabled string `json:"deviceClassEnabled"`
		Tags               struct {
		} `json:"tags"`
	} `json:"deviceInfo"`
	Level                   string  `json:"level"`
	Code                    string  `json:"code"`
	Description             string  `json:"description"`
	DevAddr                 string  `json:"devAddr"`
	Adr                     bool    `json:"adr"`
	Dr                      int     `json:"dr"`
	FCnt                    int     `json:"fCnt"`
	FPort                   int     `json:"fPort"`
	Confirmed               bool    `json:"confirmed"`
	Data                    string  `json:"data"`
	Margin                  int     `json:"margin"`
	ExternalPowerSource     bool    `json:"externalPowerSource"`
	BatteryLevelUnavailable bool    `json:"batteryLevelUnavailable"`
	BatteryLevel            float64 `json:"batteryLevel"`
	Object                  struct {
		Battery float64 `json:"battery"`
		// sensecap
		Err      float64 `json:"err"`
		Valid    bool    `json:"valid"`
		Payload  string  `json:"payload"`
		Messages []struct {
			MeasurementValue float64 `json:"measurementValue"`
			MeasurementID    float64 `json:"measurementId"`
			Battery          float64 `json:"battery"`
			Interval         float64 `json:"interval"`
			Type             string  `json:"type"`
		} `json:"messages"`
		// dragino-lht52
		TempCDS      float64 `json:"TempC_DS"`
		Ext          float64 `json:"Ext"`
		TempCSHT     float64 `json:"TempC_SHT"`
		HumSHT       float64 `json:"Hum_SHT"`
		Systimestamp float64 `json:"Systimestamp"`
		// rejee
		Vol         float64 `json:"vol"`
		Temperature float64 `json:"temperature"`
		Humidity    float64 `json:"humidity"`
		// milesight
		Position string  `json:"position"`
		Distance float64 `json:"distance"`
		Decoded  struct {
			Humidity    float64 `json:"humidity"`
			Temperature float64 `json:"temperature"`
			Battery     float64 `json:"battery"`
		} `json:"decoded"`
	} `json:"object"`
	RxInfo []RxInfoDoc `json:"rxInfo"`
	TxInfo struct {
		Frequency  int `json:"frequency"`
		Modulation struct {
			Lora struct {
				Bandwidth       int    `json:"bandwidth"`
				SpreadingFactor int    `json:"spreadingFactor"`
				CodeRate        string `json:"codeRate"`
			} `json:"lora"`
		} `json:"modulation"`
	} `json:"txInfo"`
	Context struct {
		DeduplicationID string `json:"deduplication_id"`
	} `json:"context"`
}

type RxInfoDoc struct {
	GatewayID string  `json:"gatewayId"`
	UplinkID  int     `json:"uplinkId"`
	Rssi      int     `json:"rssi"`
	Snr       float64 `json:"snr"`
	Channel   int     `json:"channel"`
	Location  struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
	Context  string `json:"context"`
	Metadata struct {
		RegionConfigID   string `json:"region_config_id"`
		RegionCommonName string `json:"region_common_name"`
	} `json:"metadata"`
	CrcStatus string `json:"crcStatus"`
}

type APIToken string

func (a APIToken) GetRequestMetadata(ctx context.Context, url ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", a),
	}, nil
}

func (a APIToken) RequireTransportSecurity() bool {
	return false
}

func getDeviceStatus() {
	if len(getApiKey()) > 0 {
		if len(labelsMap) > 0 {
			log.Debug().Msg("Using GRPC to query chirpstack for deviceStatus")
			dialOpts := []grpc.DialOption{
				grpc.WithBlock(),
				grpc.WithPerRPCCredentials(APIToken(getApiKey())),
				grpc.WithInsecure(), // remove this when using TLS
			}

			conn, dialErr := grpc.Dial(config.ApiServer, dialOpts...)
			defer conn.Close()
			if dialErr != nil {
				log.Error().Err(dialErr).Msgf("Failed to dial to %s", config.ApiServer)
				grpcConnectionErrorTotal.Inc()
				return
			}
			deviceClient := api.NewDeviceServiceClient(conn)
			for devEui := range labelsMap {
				deviceResponse, err := deviceClient.Get(context.Background(), &api.GetDeviceRequest{DevEui: devEui})
				if err != nil {
					grpcApiErrorTotal.Inc()
					delete(labelsMap, devEui)
					log.Error().Err(err).Str("devEui", devEui).Msgf("Failed to get device, will not try again for now.")
				} else {
					grpcApiTotal.Inc()
					if deviceResponse.GetDeviceStatus().GetBatteryLevel() > 0 {
						deviceBattery.With(labelsMap[devEui]).Set(float64(deviceResponse.GetDeviceStatus().GetBatteryLevel()))
					}
					if deviceResponse.GetDeviceStatus().GetExternalPowerSource() {
						deviceExternalPower.With(labelsMap[devEui]).Set(1)
					} else {
						deviceExternalPower.With(labelsMap[devEui]).Set(0)
					}

				}
			}
			grpcConnectionTotal.Inc()
		} else {
			log.Debug().Msg("No deviceEui to query device status")
		}
	} else {
		log.Debug().Msg("No API Key to getDeviceStatus")
	}

}

func getApiKey() string {
	if len(config.ApiKey) > 0 {
		return config.ApiKey
	}
	if file, err := os.Open(config.ApiFile); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Scan()
		return scanner.Text()
	}
	return ""
}

// https://sensecap-docs.seeed.cc/measurement_list.html
var senseCapMeasurementIdTypeMap = map[float64]string{
	4097: "airTemperature",
	4098: "airHumidity",
	4099: "lightIntensity",
	4100: "co2",
	4101: "barometricPressure",
}

func parseChirpstackWebhook(body []byte) (string, bool, error) {
	var payload WebHookDoc
	// set needDump to true if we need a dump, we do this so we don't dump twice
	needDump := false
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", true, err
	}
	devEui := payload.DeviceInfo.DevEui
	OUI := getOui(devEui)
	if len(payload.RxInfo) > 0 {
		for _, rxinfo := range payload.RxInfo {
			deviceGatewayLabel := prometheus.Labels{"gatewayId": rxinfo.GatewayID, "deviceName": payload.DeviceInfo.DeviceName, "deviceEui": payload.DeviceInfo.DevEui}
			deviceLastseen.With(deviceGatewayLabel).Set(float64(payload.Time.Unix()))
			deviceRxInfoRssi.With(deviceGatewayLabel).Set(float64(rxinfo.Rssi))
			deviceRxInfoSnr.With(deviceGatewayLabel).Set(float64(rxinfo.Snr))
		}
	}

	labelsMap[devEui] = prometheus.Labels{"deviceName": payload.DeviceInfo.DeviceName, "deviceEui": payload.DeviceInfo.DevEui}
	deviceLabel := labelsMap[devEui]

	if payload.BatteryLevel > 0 {
		log.Debug().Str("devEui", devEui).Msg("Got battery level")
		deviceBattery.With(labelsMap[devEui]).Set(float64(payload.BatteryLevel))
	}
	deviceFcnt.With(deviceLabel).Set(float64(payload.FCnt))
	if payload.Confirmed {
		deviceConfirmed.With(deviceLabel).Inc()
	} else {
		deviceUnconfirmed.With(deviceLabel).Inc()
	}
	if payload.Level != "" {
		deviceMsgLevelCount.With(prometheus.Labels{"deviceName": payload.DeviceInfo.DeviceName, "deviceEui": payload.DeviceInfo.DevEui, "level": payload.Level, "code": payload.Code}).Inc()
		log.Warn().Str("devEui", devEui).Str("OUI", OUI).Str("level", payload.Level).Str("code", payload.Code).Msgf("Webhook posted an error")
	}
	switch OUI {
	// Sensecap
	case "2c:f7:f1":
		// https://sensecap-docs.seeed.cc/measurement_list.html
		for _, m := range payload.Object.Messages {
			switch m.Type {
			case "report_telemetry":
				if metricType, ok := senseCapMeasurementIdTypeMap[m.MeasurementID]; ok {
					deviceLabel["type"] = metricType
					deviceMetric.With(deviceLabel).Set(float64(m.MeasurementValue))
				} else {
					log.Error().Str("DevEUI", payload.DeviceInfo.DevEui).Msgf("MeasurementId %0f is not supported", m.MeasurementID)
				}
			case "upload_battery":
				deviceLabel["type"] = "battery"
				deviceMetric.With(deviceLabel).Set(float64(m.Battery))
			case "upload_interval":
				deviceLabel["type"] = "interval"
				deviceMetric.With(deviceLabel).Set(float64(m.Interval))
			}
		}
	// REJEE
	case "ca:cb:b8":
		if payload.Object.Battery > 0 {
			deviceLabel["type"] = "battery"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Battery))
		}
		if payload.Object.Temperature > 0 {
			deviceLabel["type"] = "airTemperature"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Temperature))
		}
		if payload.Object.Humidity > 0 {
			deviceLabel["type"] = "airHumidity"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Humidity))
		}
		if payload.Object.Vol > 0 {
			deviceLabel["type"] = "vol"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Vol))
		}
	// DRAGINO
	case "a8:40:41":
		if payload.Object.TempCSHT > 0 {
			deviceLabel["type"] = "airTemperature"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.TempCSHT))
		}
		if payload.Object.TempCDS > 0 {
			deviceLabel["type"] = "externalTemperature"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.TempCDS))
		}
		if payload.Object.HumSHT > 0 {
			deviceLabel["type"] = "airHumidity"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.HumSHT))
		}
	// Milesight
	case "24:e1:24":
		if payload.Object.Decoded.Temperature > 0 {
			deviceLabel["type"] = "airTemperature"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Decoded.Temperature))
		}
		if payload.Object.Decoded.Humidity > 0 {
			deviceLabel["type"] = "airHumidity"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Decoded.Humidity))
		}
		if payload.Object.Distance > 0 {
			deviceLabel["type"] = "distance"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Distance))
		}
		if len(payload.Object.Position) > 0 {
			deviceLabel["type"] = "position"
			if payload.Object.Position == "normal" {
				deviceMetric.With(deviceLabel).Set(float64(0))
			} else { // "tilt"
				deviceMetric.With(deviceLabel).Set(float64(1))
			}
		}
		if payload.Object.Decoded.Battery > 0 {
			deviceLabel["type"] = "battery"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Decoded.Battery))
		}
		if payload.Object.Battery > 0 {
			deviceLabel["type"] = "battery"
			deviceMetric.With(deviceLabel).Set(float64(payload.Object.Battery))
		}
	default:
		needDump = true
		log.Warn().Str("devEui", devEui).Str("OUI", OUI).Msgf("Unsupported OUI")
	}
	// deviceLabel is a pointer to labelsMap[devEui] so we need to remove type which is not used by deviceLabel
	delete(deviceLabel, "type")

	log.Debug().Str("devEui", devEui).Str("OUI", OUI).Bool("needDump", needDump).Msg("Parsed Webhook")
	return devEui, needDump, nil
}

// Get OUI in XX:XX:XX hex format
func getOui(s string) string {
	if len(s) >= 6 {
		return strings.ToLower(fmt.Sprintf("%s:%s:%s", s[0:2], s[2:4], s[4:6]))
	} else {
		return "00:00:00"
	}
}
