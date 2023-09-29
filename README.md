# lora_exporter

## This assumes you are using chirpstack with docker-compose

The apikey is needed for lora_exporter to also pull device info like battery
life. This is done once and uses the grpc which is build in(not rest rpi)

```
mkdir -p /opt/chirpstack/debug
docker compose run --rm --entrypoint sh --user root chirpstack -c "chirpstack -c /etc/chirpstack create-api-key --name apiUser" | tail -n 1 | cut -d " " -f 2 | sudo tee /opt/chirpstack/apikey.txt
```

Basic setup to add in docker-compose
```
  lora-exporter:
    image: visago/lora_exporter:2023.09.28
    restart: unless-stopped
    ports:
      - 5672:5672
    volumes:
      - /tmp/lora:/tmp/lora
```

More advance use with GRPC,etc.
```
  lora-exporter:
    image: visago/lora_exporter:2023.09.28
    restart: unless-stopped
    environment:
      - APISERVER=chirpstack:8080
      - DUMP_FOLDER=/tmp/lora
      - APIFILE=/tmp/apikey.txt
    ports:
      - 5672:5672
    volumes:
      - /opt/chirpstack/debug:/tmp/lora
      - /opt/chirpstack/apikey.txt:/tmp/apikey.txt
   
```

After the above is added, `docker compose up -d`

Within chirpstack, goto integrations, add webhook of http://lora-exporter:5672

Prometheus can scrape http://lora-exporter:5672/metrics to pull metrics