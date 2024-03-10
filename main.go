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

	"github.com/gorilla/mux"
)

const serverString = "0.0.0.0:8080"

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var (
			payloadIn  interface{}
			payloadOut iMessagePayload
		)
		if err := json.Unmarshal(body, &payloadIn); err != nil {
			log.Printf("got bad req: %s\n", body)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sendTo := os.Getenv("IMESSAGE_RECIPIENT")
		if sendTo == "" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("recipient env var is not set on server!"))
			return
		}
		payloadOut = formatMessage(payloadIn, sendTo)
		if err := sendMessage(payloadOut); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
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

func formatMessage(input interface{}, sendTo string) iMessagePayload {
	out := new(iMessagePayload)
	switch in := input.(type) {
	case GrafanaAlertPayload:
		out.Body.Message = fmt.Sprintf("[%s] %s %s", in.State, in.RuleName, in.Message)
	case AlertmanagerPayload:
		for _, alert := range in.Alerts {
			out.Body.Message = fmt.Sprintf("[%s][%s] %s\n%s\n",
				alert.Labels["alertname"],
				alert.Status, alert.Annotations["description"],
				alert.GeneratorURL,
			)
		}
	}
	out.Recipient.Handle = sendTo

	return *out
}

func sendMessage(in iMessagePayload) error {
	if _, err := os.ReadFile("/tmp/pause"); err == nil {
		log.Println("Pause file detected. Won't send message!")
		return nil
	}

	jsonStr, _ := json.Marshal(in)
	req, _ := http.NewRequest("POST", "http://192.168.1.12:3005/message", bytes.NewBuffer(jsonStr))
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
	DashboardID int `json:"dashboardId"`
	EvalMatches []struct {
		Value  int    `json:"value"`
		Metric string `json:"metric"`
		Tags   struct {
		} `json:"tags"`
	} `json:"evalMatches"`
	ImageURL string `json:"imageUrl"`
	Message  string `json:"message"`
	OrgID    int    `json:"orgId"`
	PanelID  int    `json:"panelId"`
	RuleID   int    `json:"ruleId"`
	RuleName string `json:"ruleName"`
	RuleURL  string `json:"ruleUrl"`
	State    string `json:"state"`
	Tags     struct {
		TagName string `json:"tag name"`
	} `json:"tags"`
	Title string `json:"title"`
}
