package main

import (
	"database/sql"
	"encoding/json"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var db *sql.DB

type MetricsRequestBody struct {
	ResourceID string             `json:"resource_id"`
	Timestamp  time.Time          `json:"timestamp"`
	Metrics    map[string]float64 `json:"metrics"`
}

func handlerMetricsPush(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var body MetricsRequestBody
	err := decoder.Decode(&body)
	if err != nil {
		log.Print(err)
		http.Error(w, "Could not parse JSON", 400)
		return
	}
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
	DataPoints []GraphDataPoint
}

func handlerGraph(w http.ResponseWriter, r *http.Request) {
	var params GraphTemplateParams

	params.ResourceID = r.FormValue("resource_id")
	params.MetricKey = r.FormValue("metric_key")

	change, _ := strconv.ParseBool(r.FormValue("change"))

	rows, err := db.Query("SELECT * FROM (SELECT timestamp, value FROM datapoints WHERE resource_id = $1 AND metric_key = $2 ORDER BY timestamp DESC LIMIT 100) s ORDER BY timestamp ASC", params.ResourceID, params.MetricKey)
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

	r := mux.NewRouter()
	r.HandleFunc("/metrics", handlerMetricsPush).Methods("POST")
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
