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
