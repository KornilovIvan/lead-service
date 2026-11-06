package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type lead struct {
	ID       int    `json:"ID"`
	OriginID string `json:"ORIGIN_ID"`
	Name     string `json:"NAME"`
	Phone    string `json:"PHONE"`
	Email    string `json:"EMAIL"`
}

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	var delay time.Duration
	if raw := os.Getenv("ADD_DELAY"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			log.Fatal(err)
		}
		delay = parsed
	}

	var (
		mu    sync.Mutex
		leads []lead
		next  = 1
	)

	r := chi.NewRouter()
	r.Post("/rest/{user}/{token}/crm.lead.add.json", func(w http.ResponseWriter, req *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		if status, ok := forcedStatus(req); ok {
			w.WriteHeader(status)
			return
		}

		var body struct {
			Fields struct {
				OriginID string `json:"ORIGIN_ID"`
				Name     string `json:"NAME"`
				Phone    []struct {
					Value string `json:"VALUE"`
				} `json:"PHONE"`
				Email []struct {
					Value string `json:"VALUE"`
				} `json:"EMAIL"`
			} `json:"fields"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		item := lead{OriginID: body.Fields.OriginID, Name: body.Fields.Name}
		if len(body.Fields.Phone) > 0 {
			item.Phone = body.Fields.Phone[0].Value
		}
		if len(body.Fields.Email) > 0 {
			item.Email = body.Fields.Email[0].Value
		}

		mu.Lock()
		item.ID = next
		next++
		leads = append(leads, item)
		mu.Unlock()

		writeJSON(w, map[string]int{"result": item.ID})
	})
	r.Post("/rest/{user}/{token}/crm.lead.list.json", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Filter struct {
				OriginID string `json:"ORIGIN_ID"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		mu.Lock()
		matched := make([]lead, 0)
		for _, item := range leads {
			if item.OriginID == body.Filter.OriginID {
				matched = append(matched, item)
			}
		}
		mu.Unlock()

		writeJSON(w, map[string]any{"result": matched})
	})

	log.Fatal(http.ListenAndServe(addr, r))
}

func forcedStatus(r *http.Request) (int, bool) {
	raw := r.Header.Get("X-Force-Status")
	if raw == "" {
		return 0, false
	}
	status, err := strconv.Atoi(raw)
	if err != nil {
		return http.StatusBadRequest, true
	}
	return status, true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
