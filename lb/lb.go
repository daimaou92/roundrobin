package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Instance struct {
	mx                   sync.Mutex
	url                  string
	avgResponseTimeMilli float64
	responseTimeCache    []int64 // Store a window of response times to create average
	lastResponseAt       int64
	healthy              bool
	cancelFunc           context.CancelFunc
}

func NewInstance(urlString string) (*Instance, error) {
	urlAddr, err := url.Parse(urlString)
	if err != nil {
		return nil, fmt.Errorf("[NewInstance] -> malformed url: %s", err.Error())
	}

	if urlAddr.Scheme != "http" {
		return nil, fmt.Errorf("[NewInstance] -> Invalid url protocol. Expected: `http`. Actual: `%s`", urlAddr.Scheme)
	}
	return &Instance{
		url: fmt.Sprintf("%s://%s", urlAddr.Scheme, urlAddr.Host),
	}, nil
}

func (ins *Instance) monitor(ctx context.Context) {
	tc := time.NewTicker(time.Second * 1)   // Check instance health every second
	tAvg := time.NewTicker(time.Second * 2) // Calculate response time average every 2 seconds
	for {
		select {
		case <-ctx.Done():
			return
		case <-tc.C:
			client := http.Client{Timeout: time.Millisecond * 10}
			_, err := client.Get(fmt.Sprintf("%s/health", ins.url))
			if err != nil {
				time.Sleep(time.Millisecond * 100)
				_, err = client.Get(fmt.Sprintf("%s/health", ins.url))
				if err != nil {
					ins.mx.Lock()
					ins.healthy = false
					ins.mx.Unlock()
				}
			} else {
				ins.mx.Lock()
				ins.healthy = true
				ins.mx.Unlock()
			}
		case <-tAvg.C:
			if (time.Now().UnixMilli() - ins.lastResponseAt) > 5*1000 { // Every 5+ seconds allow the server to be called again
				ins.mx.Lock()
				ins.avgResponseTimeMilli = 0
				ins.responseTimeCache = []int64{}
				ins.mx.Unlock()
			}

			if len(ins.responseTimeCache) > 0 {
				sum := int64(0)
				ins.mx.Lock()
				for _, v := range ins.responseTimeCache {
					sum += v
				}
				ins.avgResponseTimeMilli = float64(sum) / float64(len(ins.responseTimeCache))
				ins.mx.Unlock()
			}
			RESPONSE_DURATION_METRIC.WithLabelValues(ins.url).Set(ins.avgResponseTimeMilli)
		}

	}
}

func (ins *Instance) isAvailable() bool {
	if ins.avgResponseTimeMilli > 10 {
		return false
	}

	return ins.healthy
}

func (ins *Instance) logResponseTime(start, end int64) {
	ins.mx.Lock()
	defer ins.mx.Unlock()

	ins.responseTimeCache = append(ins.responseTimeCache, (end - start))
	if len(ins.responseTimeCache) > 20 {
		ins.responseTimeCache = ins.responseTimeCache[1:]
	}
	ins.lastResponseAt = end
}

func (ins *Instance) jsonHandler(res http.ResponseWriter, req *http.Request) error {
	start := time.Now().UnixMilli()
	// call the associated responder service
	client := http.Client{Timeout: time.Second * 200}
	resp, err := client.Post(fmt.Sprintf("%s/json", ins.url), "application/json", req.Body)
	end := time.Now().UnixMilli()

	// Log avg response time in go routine so as to not block the response
	go ins.logResponseTime(start, end)

	if err != nil {
		return fmt.Errorf("[Instance.jsonHandler] -> Error calling responder: %s", err)
	}

	res.WriteHeader(resp.StatusCode)
	RESPONSE_STATUS_METRIC.WithLabelValues(fmt.Sprintf("%d", resp.StatusCode)).Inc()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	log.Printf("[Instance.jsonHandler] -> responding from: `%s`\n", ins.url)
	res.Header().Set("Content-Type", "application/json")
	io.Copy(res, resp.Body)
	return nil
}

type LB struct {
	mx        sync.Mutex
	instances []*Instance
	current   int
	Ctx       context.Context
}

func NewLB(ctx context.Context, instanceURLList string) (*LB, error) { // arugument is a comma separated string
	lb := &LB{Ctx: ctx}
	if instanceURLList == "" {
		return lb, nil
	}
	instanceURLArr := strings.Split(instanceURLList, ",")
	for _, instanceURL := range instanceURLArr {
		if err := lb.AddInstance(instanceURL); err != nil {
			return nil, fmt.Errorf("[NewLB] -> %s", err.Error())
		}
	}
	return lb, nil
}

func (lb *LB) AddInstance(url string) error {
	instance, err := NewInstance(url)
	if err != nil {
		return fmt.Errorf("[LB.AddInstance] -> %s", err.Error())
	}
	ctx, cancel := context.WithCancel(lb.Ctx)
	lb.mx.Lock()
	lb.instances = append(lb.instances, instance)
	instance.cancelFunc = cancel
	lb.mx.Unlock()
	go instance.monitor(ctx)
	return nil
}

func (lb *LB) RemoveInstance(url string) {
	lb.mx.Lock()
	defer lb.mx.Unlock()
	var instance *Instance = nil
	var instanceIndex = -1
	for i, ins := range lb.instances {
		if strings.HasPrefix(url, ins.url) {
			instance = ins
			instanceIndex = i
			break
		}
	}

	if instance != nil {
		// cancel monitoring
		instance.cancelFunc()
		lb.instances = append(lb.instances[0:instanceIndex], lb.instances[instanceIndex+1:]...)
	}
}

// Round Robin - kinda!
func (lb *LB) GetInstance() *Instance {
	lb.mx.Lock()
	defer lb.mx.Unlock()

	n := len(lb.instances)
	checked := 0

	// This should never happen but oh well
	if n == 0 {
		return nil
	}

	// Start looking at available nodes from 1 + the last node used
	// Increment by 1 until a node is found or we have checked all listed nodes
	index := lb.current + 1
	if index >= n {
		index = 0
	}
	for checked < n {
		checked += 1
		if lb.instances[index].isAvailable() {
			lb.current = index
			return lb.instances[index]
		}
		index += 1
		if index >= n {
			index = 0
		}
	}

	// Return empty if no available node was found
	return nil
}
