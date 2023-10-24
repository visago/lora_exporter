curl -d "@${1:-sample/sensecapCo2TempHumidSensor.json}" -H "Content-Type: application/json" -X POST 127.0.0.1:5672/
echo ""
curl -s 127.0.0.1:5672/metrics | grep -i "^lora"
