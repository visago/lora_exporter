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

type WebHookSensecapMessage struct {
	MeasurementValue json.Number `json:"measurementValue"`
	MeasurementID    json.Number `json:"measurementId"`
	Battery          json.Number `json:"battery"`
	Interval         json.Number `json:"interval"`
	Type             string      `json:"type"`
}

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
		Err      null.Float      `json:"err"`
		Valid    bool            `json:"valid"`
		Payload  string          `json:"payload"`
		Messages json.RawMessage `json:"messages"`
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
				log.Error().Caller().Err(dialErr).Msgf("Failed to dial to %s", config.ApiServer)
				grpcConnectionErrorTotal.Inc()
				return
			}
			deviceClient := api.NewDeviceServiceClient(conn)
			for devEui := range labelsMap {
				deviceResponse, err := deviceClient.Get(context.Background(), &api.GetDeviceRequest{DevEui: devEui})
				if err != nil {
					grpcApiErrorTotal.Inc()
					delete(labelsMap, devEui)
					log.Error().Caller().Err(err).Str("devEui", devEui).Msgf("Failed to get device, will not try again for now.")
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
	3000: "battery",
	3940: "sosMode",
	3941: "workMode",
	4097: "airTemperature",
	4098: "airHumidity",
	4099: "lightIntensity",
	4100: "co2",
	4101: "barometricPressure",
	4102: "soilTemperature",
	4103: "soilMoisture",
	4104: "windDirection",
	4105: "windSpeed",
	4106: "pH",
	4107: "lightQuantum",
	4108: "electricalConductivity",
	4109: "dissolvedOxygen",
	4204: "soilPoreWaterEletricalConductivity",
	4205: "epsilon",
	4197: "longitude",
	4198: "latitude",
	4199: "lightIntensityPercent",
	4200: "sosEvent",
}

func parseChirpstackWebhook(body []byte) (string, bool, error) {
	var payload WebHookDoc
	var payloadSensecap []WebHookSensecapMessage
	var lastLat, lastLon float64
	metricGeoFlag := false

	// set needDump to true if we need a dump, we do this so we don't dump twice
	needDump := false
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", true, err
	}
	if len(payload.Object.Messages) > 0 {
		// This is a sensecap device
		if err := json.Unmarshal(payload.Object.Messages, &payloadSensecap); err != nil {
			// If it fails to unmarshal its because its an array of array of messages. So we dearray it
			var deArray []json.RawMessage
			if err := json.Unmarshal(payload.Object.Messages, &deArray); err != nil {
				// Its not an array of array i guess :P return error
				return "", true, err
			}
			for _, rawJson := range deArray {
				var newMessages []WebHookSensecapMessage
				if err := json.Unmarshal(rawJson, &newMessages); err != nil {
					return "", true, err
				}
				payloadSensecap = append(payloadSensecap, newMessages...)
			}
		}
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

	// We check if this is the first time
	_, proccesedBefore := labelsMap[devEui]
	if !proccesedBefore {
		log.Info().Str("deviceName", payload.DeviceInfo.DeviceName).Str("deviceEui", payload.DeviceInfo.DevEui).Msg("First time procesing this deviceEUI, dumping in case.")
		needDump = true
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
		for _, m := range payloadSensecap {
			switch m.Type {
			case "upload_battery":
				deviceLabel["type"] = "battery"
				deviceMetric.With(deviceLabel).Set(castToFloat64(m.Battery))
			case "upload_interval":
				deviceLabel["type"] = "interval"
				deviceMetric.With(deviceLabel).Set(castToFloat64(m.Interval))
			default:
				id := castToFloat64(m.MeasurementID)
				if id > 0 {
					if metricType, ok := senseCapMeasurementIdTypeMap[id]; ok {
						deviceLabel["type"] = metricType
						deviceMetric.With(deviceLabel).Set(castToFloat64(m.MeasurementValue))
						if metricType == "longitude" {
							metricGeoFlag = config.MetricsGeo
							lastLon = castToFloat64(m.MeasurementValue)
						} else if metricType == "latitude" {
							metricGeoFlag = config.MetricsGeo
							lastLat = castToFloat64(m.MeasurementValue)
						}
					} else {
						log.Error().Caller().Str("DevEUI", payload.DeviceInfo.DevEui).Msgf("MeasurementId %0f is not supported", m.MeasurementID)
					}
				}
			}

		}
		if metricGeoFlag {
			// We have to loop through the data again to create new metrics with geo data
			for _, m := range payloadSensecap {
				id := castToFloat64(m.MeasurementID)
				if id > 0 {
					if metricType, ok := senseCapMeasurementIdTypeMap[id]; ok {
						if metricType != "longitude" && metricType != "latitude" {
							label := prometheus.Labels{"deviceName": payload.DeviceInfo.DeviceName, "deviceEui": payload.DeviceInfo.DevEui, "type": metricType, "lat": fmt.Sprintf("%f", lastLat), "lon": fmt.Sprintf("%f", lastLon)}
							deviceMetricGeo.With(label).Set(castToFloat64(m.MeasurementValue))
						}
					}
				}
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
		if payload.Object.Temperature.Valid {
			deviceLabel["type"] = "temperature" // We use temperatuer when we don't know if its for liquid or air
			deviceMetric.With(deviceLabel).Set(payload.Object.Temperature.Float64)
		}
		if payload.Object.Humidity.Valid {
			deviceLabel["type"] = "airHumidity"
			deviceMetric.With(deviceLabel).Set(payload.Object.Humidity.Float64)
		}
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

// This casts json.Nubmer without the error
func castToFloat64(num json.Number) float64 {
	f, _ := num.Float64()
	return f
}
