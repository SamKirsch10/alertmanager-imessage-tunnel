package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
)

const serverString = "0.0.0.0:8080"

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/grafana", func(w http.ResponseWriter, r *http.Request) {
		var payloadIn GrafanaAlertPayload
		processRequest(&payloadIn, w, r)
	})
	r.HandleFunc("/alertmanager", func(w http.ResponseWriter, r *http.Request) {
		var payloadIn AlertmanagerPayload
		processRequest(&payloadIn, w, r)
	})

	srv := http.Server{
		Handler:      r,
		Addr:         serverString,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	log.Printf("Starting server: %s\n", serverString)

	log.Fatal(srv.ListenAndServe())
}

func formatMessage(input interface{}, sendTo string) (iMessagePayload, error) {
	out := new(iMessagePayload)
	switch i := input.(type) {
	case *AlertmanagerPayload:
		log.Println("detected alertmanager payload")
		for _, alert := range i.Alerts {
			out.Body.Message += fmt.Sprintf("[%s][%s] %s\n%s\n",
				alert.Labels["alertname"],
				alert.Status, alert.Annotations["description"],
				alert.GeneratorURL,
			)
		}
	case *GrafanaAlertPayload:
		log.Println("detected grafana alert payload")
		for _, alert := range i.Alerts {
			log.Printf("processing alert %s", alert.Labels["alertname"])
			out.Body.Message += fmt.Sprintf("[%s] %s %s\n", alert.Status, alert.Labels["alertname"], alert.GeneratorURL)
		}
	default:
		j, _ := json.Marshal(input)
		log.Printf("\n--------------\n%s\n--------------\n", j)
		return *out, fmt.Errorf("unknown type %T received. or not alerts found. cannot build msg", i)
	}
	out.Recipient.Handle = sendTo

	return *out, nil
}

func sendMessage(in iMessagePayload) error {
	if _, err := os.ReadFile("/tmp/pause"); err == nil {
		log.Println("Pause file detected. Won't send message!")
		return nil
	}

	jsonStr, _ := json.Marshal(in)
	req, err := http.NewRequest("POST", "http://192.168.1.12:3005/message", bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from imessage server: %s", body)
	}
	return nil
}

func processRequest(payloadIn interface{}, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &payloadIn); err != nil {
		log.Printf("got bad req: %v\n%s\n", err, body)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sendTo := os.Getenv("IMESSAGE_RECIPIENT")
	if sendTo == "" {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("recipient env var is not set on server!"))
		return
	}
	payloadOut, err := formatMessage(payloadIn, sendTo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("%v", err)))
		log.Printf("error formatting msg: %v\n", err)
		return
	}
	spew.Dump(payloadOut)
	if err := sendMessage(payloadOut); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error sending msg: %v\n", err)
		w.Write([]byte(err.Error()))
	}
}

type iMessagePayload struct {
	Body struct {
		Message string `json:"message"`
	} `json:"body"`
	Recipient struct {
		Handle string `json:"handle"`
	} `json:"recipient"`
}

type AlertmanagerPayload struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []struct {
		Status       string            `json:"status"`
		Labels       map[string]string `json:"labels"`
		Annotations  map[string]string `json:"annotations"`
		StartsAt     string            `json:"startsAt"`
		EndsAt       string            `json:"endsAt"`
		GeneratorURL string            `json:"generatorURL"`
		Fingerprint  string            `json:"fingerprint"`
	} `json:"alerts"`
}

type GrafanaAlertPayload struct {
	Receiver string `json:"receiver"`
	Status   string `json:"status"`
	OrgID    int    `json:"orgId"`
	Alerts   []struct {
		Status       string            `json:"status"`
		Labels       map[string]string `json:"labels"`
		Annotations  map[string]string `json:"annotations"`
		StartsAt     string            `json:"startsAt"`
		EndsAt       string            `json:"endsAt"`
		GeneratorURL string            `json:"generatorURL"`
		Fingerprint  string            `json:"fingerprint"`
		SilenceURL   string            `json:"silenceURL"`
		DashboardURL string            `json:"dashboardURL"`
		PanelURL     string            `json:"panelURL"`
		Values       map[string]any    `json:"values"`
	} `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
	Title             string            `json:"title"`
	State             string            `json:"state"`
	Message           string            `json:"message"`
}
