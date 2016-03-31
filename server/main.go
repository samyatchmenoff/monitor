package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var db *sql.DB
var triggerReqChan chan MetricsPostRequestBody = make(chan MetricsPostRequestBody)

type ResourcesPostRequestBody struct {
	ResourceID string `json:"resource_id"`
	Tags       map[string]string
	Hostname   string
}

func handlerResourcesPost(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var body ResourcesPostRequestBody
	err := decoder.Decode(&body)
	if err != nil {
		log.Print(err)
		http.Error(w, "Could not parse JSON", 400)
		return
	}
}

type MetricsPostRequestBody struct {
	ResourceID string             `json:"resource_id"`
	Timestamp  time.Time          `json:"timestamp"`
	Metrics    map[string]float64 `json:"metrics"`
}

func handlerMetricsPost(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var body MetricsPostRequestBody
	err := decoder.Decode(&body)
	if err != nil {
		log.Print(err)
		http.Error(w, "Could not parse JSON", 400)
		return
	}
	triggerReqChan <- body
	for k, v := range body.Metrics {
		_, err := db.Exec("INSERT INTO datapoints (resource_id, metric_key, timestamp, value) VALUES ($1, $2, $3, $4)",
			body.ResourceID, k, body.Timestamp, v)
		if err != nil {
			log.Print(err)
			http.Error(w, "Internal Server Error", 500)
			return
		}
	}
}

type RootTemplateParams struct {
	Metrics map[string][]string
}

func handlerRoot(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT DISTINCT resource_id, metric_key FROM datapoints ORDER BY resource_id, metric_key")
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	defer rows.Close()
	var params RootTemplateParams
	params.Metrics = make(map[string][]string)
	for rows.Next() {
		var resourceID string
		var metricKey string
		if err := rows.Scan(&resourceID, &metricKey); err != nil {
			log.Print(err)
			http.Error(w, "Internal Server Error", 500)
			return
		}
		params.Metrics[resourceID] = append(params.Metrics[resourceID], metricKey)
	}

	t, err := template.ParseFiles("templates/root.html")
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	err = t.Execute(w, params)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
}

type GraphDataPoint struct {
	X int64   `json:"x"`
	Y float64 `json:"y"`
}

type GraphTemplateParams struct {
	ResourceID string
	MetricKey  string
	Aggregator string
	DataPoints []GraphDataPoint
}

func handlerGraph(w http.ResponseWriter, r *http.Request) {
	var params GraphTemplateParams

	params.ResourceID = r.FormValue("resource_id")
	params.MetricKey = r.FormValue("metric_key")
	params.Aggregator = r.FormValue("aggregator")
	if len(params.Aggregator) == 0 {
		params.Aggregator = "avg"
	}
	var aggregatorFunc string
	switch params.Aggregator {
	case "avg":
		aggregatorFunc = "avg"
	case "sum":
		aggregatorFunc = "sum"
	case "max":
		aggregatorFunc = "max"
	case "min":
		aggregatorFunc = "min"
	default:
		aggregatorFunc = "avg"
		params.Aggregator = "avg"
	}

	change, _ := strconv.ParseBool(r.FormValue("change"))

	var rows *sql.Rows
	var err error

	if len(params.ResourceID) == 0 {
		rows, err = db.Query(fmt.Sprintf("SELECT timestamp, %s(value) FROM (SELECT date_trunc('minute', timestamp) AS timestamp, value FROM datapoints WHERE metric_key LIKE $1 AND timestamp > now() - INTERVAL '3 hours' ORDER BY timestamp DESC) s GROUP BY timestamp ORDER BY timestamp ASC", aggregatorFunc), strings.Replace(params.MetricKey, "*", "%", -1))
	} else {
		rows, err = db.Query(fmt.Sprintf("SELECT timestamp, %s(value) FROM (SELECT date_trunc('minute', timestamp) AS timestamp, value FROM datapoints WHERE resource_id LIKE $1 AND metric_key LIKE $2 AND timestamp > now() - INTERVAL '3 hours' ORDER BY timestamp DESC) s GROUP BY timestamp ORDER BY timestamp ASC", aggregatorFunc), strings.Replace(params.ResourceID, "*", "%", -1), strings.Replace(params.MetricKey, "*", "%", -1))
	}
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var timestamp time.Time
		var value float64
		if err := rows.Scan(&timestamp, &value); err != nil {
			log.Print(err)
			http.Error(w, "Internal Server Error", 500)
			return
		}
		params.DataPoints = append(params.DataPoints, GraphDataPoint{
			X: timestamp.Unix(),
			Y: value,
		})
	}

	if change {
		for i := 0; i < len(params.DataPoints)-1; i++ {
			params.DataPoints[i].Y = params.DataPoints[i+1].Y - params.DataPoints[i].Y
		}
		params.DataPoints = params.DataPoints[:len(params.DataPoints)-1]
	}

	t, err := template.ParseFiles("templates/graph.html")
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	err = t.Execute(w, params)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
}

func main() {
	var err error
	db, err = sql.Open("postgres", "user=samy dbname=monitor sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	go CheckTriggers(triggerReqChan)

	r := mux.NewRouter()
	r.HandleFunc("/resources", handlerResourcesPost).Methods("POST")
	r.HandleFunc("/metrics", handlerMetricsPost).Methods("POST")
	r.HandleFunc("/", handlerRoot).Methods("GET")
	r.HandleFunc("/graph", handlerGraph).Methods("GET")
	r.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/", handlers.LoggingHandler(os.Stdout, r))
	log.Print("Starting web server")
	err = http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
