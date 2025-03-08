## Prometheus
95th Percentile of Response Time - since I couldn't get it to show on grafana

```promql
histogram_quantile(0.95, sum(rate(avg_response_duration_millis_bucket[5m])) by (le))
```

## Stress responder3

- Log into the container
```bash
docker exec -it responder3 bash
```

- Stress
```bash
/stress-ng --cpu 16 --cpu-method fft --timeout 5m
```

## Add instance

```bash
curl -X PUT --data 'http://responder4:20000' localhost:30000/addinstance
```
