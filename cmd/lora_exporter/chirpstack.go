package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/chirpstack/chirpstack/api/go/v4/api"
	"github.com/guregu/null"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"os"
	"strings"
	"time"
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
		Battery null.Float `json:"battery"`
		// sensecap
		Err      null.Float `json:"err"`
		Valid    bool       `json:"valid"`
		Payload  string     `json:"payload"`
		Messages []struct {
			MeasurementValue float64 `json:"measurementValue"`
			MeasurementID    float64 `json:"measurementId"`
			Battery          float64 `json:"battery"`
			Interval         float64 `json:"interval"`
			Type             string  `json:"type"`
		} `json:"messages"`
		// dragino-lht52
		TempCDS      null.Float `json:"TempC_DS"`
		Ext          null.Float `json:"Ext"`
		TempCSHT     null.Float `json:"TempC_SHT"`
		HumSHT       null.Float `json:"Hum_SHT"`
		Systimestamp null.Float `json:"Systimestamp"`
		// dragino-ld02 (door sensor)
		LastDoorOpenDuration null.Float `json:"LAST_DOOR_OPEN_DURATION"`
		Alarm                null.Float `json:"ALARM"`
		DoorOpenTimes        null.Float `json:"DOOR_OPEN_TIMES"`
		BatV                 null.Float `json:"BAT_V"`
		Mod                  null.Float `json:"MOD"`
		DoorOpenStatus       null.Float `json:"DOOR_OPEN_STATUS"`
		// rejee
		Vol         null.Float `json:"vol"`
		Temperature null.Float `json:"temperature"`
		Humidity    null.Float `json:"humidity"`
		// milesight
		Position string     `json:"position"`
		Distance null.Float `json:"distance"`
		Decoded  struct {
			Humidity    null.Float `json:"humidity"`
			Temperature null.Float `json:"temperature"`
			Battery     null.Float `json:"battery"`
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
		if payload.Object.Battery.Valid {
			deviceLabel["type"] = "battery"
			deviceMetric.With(deviceLabel).Set(payload.Object.Battery.Float64)
		}
		if payload.Object.Temperature.Valid {
			deviceLabel["type"] = "airTemperature"
			deviceMetric.With(deviceLabel).Set(payload.Object.Temperature.Float64)
		}
		if payload.Object.Humidity.Valid {
			deviceLabel["type"] = "airHumidity"
			deviceMetric.With(deviceLabel).Set(payload.Object.Humidity.Float64)
		}
		if payload.Object.Vol.Valid {
			deviceLabel["type"] = "vol"
			deviceMetric.With(deviceLabel).Set(payload.Object.Vol.Float64)
		}
	// DRAGINO
	case "a8:40:41":
		if payload.Object.TempCSHT.Valid {
			deviceLabel["type"] = "airTemperature"
			deviceMetric.With(deviceLabel).Set(payload.Object.TempCSHT.Float64)
		}
		if payload.Object.TempCDS.Valid {
			deviceLabel["type"] = "externalTemperature"
			deviceMetric.With(deviceLabel).Set(payload.Object.TempCDS.Float64)
		}
		if payload.Object.HumSHT.Valid {
			deviceLabel["type"] = "airHumidity"
			deviceMetric.With(deviceLabel).Set(payload.Object.HumSHT.Float64)
		}
		if payload.Object.LastDoorOpenDuration.Valid {
			deviceLabel["type"] = "lastOpenDuration"
			deviceMetric.With(deviceLabel).Set(payload.Object.LastDoorOpenDuration.Float64)
		}
		if payload.Object.Alarm.Valid {
			deviceLabel["type"] = "alarm"
			deviceMetric.With(deviceLabel).Set(payload.Object.Alarm.Float64)
		}
		if payload.Object.DoorOpenTimes.Valid {
			deviceLabel["type"] = "openCount"
			deviceMetric.With(deviceLabel).Set(payload.Object.DoorOpenTimes.Float64)
		}
		if payload.Object.BatV.Valid {
			deviceLabel["type"] = "batteryVolts"
			deviceMetric.With(deviceLabel).Set(payload.Object.BatV.Float64)
		}
		if payload.Object.Mod.Valid {
			deviceLabel["type"] = "mod"
			deviceMetric.With(deviceLabel).Set(payload.Object.Mod.Float64)
		}
		if payload.Object.DoorOpenStatus.Valid {
			deviceLabel["type"] = "openStatus"
			deviceMetric.With(deviceLabel).Set(payload.Object.DoorOpenStatus.Float64)
		}
	// Milesight
	case "24:e1:24":
		if payload.Object.Decoded.Temperature.Valid {
			deviceLabel["type"] = "airTemperature"
			deviceMetric.With(deviceLabel).Set(payload.Object.Decoded.Temperature.Float64)
		}
		if payload.Object.Decoded.Humidity.Valid {
			deviceLabel["type"] = "airHumidity"
			deviceMetric.With(deviceLabel).Set(payload.Object.Decoded.Humidity.Float64)
		}
		if payload.Object.Distance.Valid {
			deviceLabel["type"] = "distance"
			deviceMetric.With(deviceLabel).Set(payload.Object.Distance.Float64)
		}
		if len(payload.Object.Position) > 0 {
			deviceLabel["type"] = "position"
			if payload.Object.Position == "normal" {
				deviceMetric.With(deviceLabel).Set(float64(0))
			} else { // "tilt"
				deviceMetric.With(deviceLabel).Set(float64(1))
			}
		}
		if payload.Object.Decoded.Battery.Valid {
			deviceLabel["type"] = "battery"
			deviceMetric.With(deviceLabel).Set(payload.Object.Decoded.Battery.Float64)
		}
		if payload.Object.Battery.Valid {
			deviceLabel["type"] = "battery"
			deviceMetric.With(deviceLabel).Set(payload.Object.Battery.Float64)
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
