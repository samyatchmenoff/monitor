package main

import (
	"log"
	"time"
)

type Alert struct {
	ResourceID string    `json:"resource_id"`
	MetricKey  string    `json:"metric_key"`
	Value      float64   `json:"value"`
	Timestamp  time.Time `json:"timestamp"`
}

func CheckTriggers(reqs chan MetricsPostRequestBody) {
	for req := range reqs {
		for k, v := range req.Metrics {
			if k == "cpu.user" && v > 15 {
				alert := Alert{
					ResourceID: req.ResourceID,
					MetricKey:  k,
					Value:      v,
					Timestamp:  req.Timestamp,
				}
				log.Print(alert)
			}
		}
	}
}
