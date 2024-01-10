package dataservice

import (
	"changestreamwebsocket/logging"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Metric struct {
	Identifier  string      `json:"ident"`
	MetricName  string      `json:"metricName"`
	MetricValue interface{} `json:"metricValue"`
	MetricTrend string      `json:"metricTrend"`
	Timestamp   time.Time   `json:"_ts"`
}

type Metrics struct {
	MetricsList   map[string]Metric `json:"metricsList"`
	StatusMessage string            `json:"statusMessage"`
}

var mutex sync.Mutex

type DataPool struct {
	Broadcast     chan Metrics
	Update        chan Metrics
	Resend        chan bool
	masterMetrics Metrics
}

func NewDataPool() *DataPool {
	return &DataPool{
		Broadcast: make(chan Metrics),
		Update:    make(chan Metrics),
		Resend:    make(chan bool),
	}
}

func (dpool *DataPool) Start() {
	mutex.Lock()
	dpool.masterMetrics.MetricsList = make(map[string]Metric)
	mutex.Unlock()
	for {
		select {
		case metricsList := <-dpool.Update:
			//Add updates to master metrics list. If changes warrant it, trigger a broadcast to listeners
			if update := dpool.assessUpdates(metricsList); update {
				dpool.Broadcast <- dpool.masterMetrics
			}
		case <-dpool.Resend:
			//A new client has connected - resend all metrics
			dpool.Broadcast <- dpool.masterMetrics
		}
	}
}

func (dpool *DataPool) Purge() {
	for {
		changed := false
		mutex.Lock()
		tempList := dpool.masterMetrics.MetricsList
		for key, metric := range dpool.masterMetrics.MetricsList {
			if metric.Timestamp.Before(time.Now().Add(time.Second * -60)) {
				//this metric hasn't been updated for more than 90 seconds - delete it.
				delete(tempList, key)
				changed = true
			}
		}
		if changed {
			//One or more metrics have timeed out and been removed
			dpool.masterMetrics.MetricsList = tempList
			dpool.Broadcast <- dpool.masterMetrics
		}
		mutex.Unlock()
		time.Sleep(time.Second)
	}
}

func (dpool *DataPool) assessUpdates(newMetrics Metrics) bool {

	//compare the new metrics with existing entries and decide if we need to broadcast an update
	changed := false
	for newKey, newMetric := range newMetrics.MetricsList {
		oldMetric, ok := dpool.masterMetrics.MetricsList[newKey]
		if !ok {
			//No metric with this identifier currently in the master list so broadcast
			mutex.Lock()
			dpool.masterMetrics.MetricsList[newKey] = newMetric
			mutex.Unlock()
			changed = true
		} else {
			//A metric with this identifier does exist
			trend, err := getTrend(newMetric, oldMetric)
			if err != nil {
				logging.ErrorLogger.Fatal(err)
			}
			newMetric.MetricTrend = trend
			mutex.Lock()
			dpool.masterMetrics.MetricsList[newKey] = newMetric
			mutex.Unlock()
			if trend != "" {
				changed = true
			}
		}
	}
	return changed
}

func GetPoolBytes(dpool *DataPool) ([]byte, error) {

	mutex.Lock()
	metricBytes, err := json.Marshal(dpool.masterMetrics)
	mutex.Unlock()
	return metricBytes, err
}

func getTrend(newMetric, oldMetric Metric) (string, error) {
	var newVal = newMetric.MetricValue
	var oldVal = oldMetric.MetricValue
	switch newVal.(type) {
	case int, int8, int16, int32, int64:
		newNum, ok := newVal.(int64)
		if !ok {
			return "", fmt.Errorf("issue interpreting new metric valuesas int64")
		}
		oldNum, ok := oldVal.(int64)
		if !ok {
			return "", fmt.Errorf("issue interpreting old metric value as int64")
		}
		if newNum > oldNum {
			return "increasing", nil
		} else if newNum < oldNum {
			return "decreasing", nil
		} else {
			return "", nil
		}
	case uint, uint8, uint16, uint32, uint64:
		newNum, ok := newVal.(uint64)
		if !ok {
			return "", fmt.Errorf("issue interpreting new metric value as uint64")
		}
		oldNum, ok := oldVal.(uint64)
		if !ok {
			return "", fmt.Errorf("issue interpreting old metric value as uint64")
		}
		if newNum > oldNum {
			return "increasing", nil
		} else if newNum < oldNum {
			return "decreasing", nil
		} else {
			return "", nil
		}
	case float32, float64:
		newNum, ok := newVal.(float64)
		if !ok {
			return "", fmt.Errorf("issue interpreting new metric value as float64")
		}
		oldNum, ok := oldVal.(float64)
		if !ok {
			return "", fmt.Errorf("issue interpreting old metric value as float64")
		}
		if newNum > oldNum {
			return "increasing", nil
		} else if newNum < oldNum {
			return "decreasing", nil
		} else {
			return "", nil
		}
	case string:
		newString, ok := newVal.(string)
		if !ok {
			return "", fmt.Errorf("issue interpreting new metric value as string")
		}
		oldString, ok := oldVal.(string)
		if !ok {
			return "", fmt.Errorf("issue interpreting old metric value as string")
		}
		if newString != oldString {
			return "changed", nil
		} else {
			return "", nil
		}
	case bool:
		newBool, ok := newVal.(bool)
		if !ok {
			return "", fmt.Errorf("issue interpreting new metric value as boolean")
		}
		oldBool, ok := oldVal.(bool)
		if !ok {
			return "", fmt.Errorf("issue interpreting old metric value as boolean")
		}
		if newBool != oldBool {
			return "changed", nil
		} else {
			return "", nil
		}
	default:
		return "", nil
	}
}
