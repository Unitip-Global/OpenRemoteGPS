package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type traccarDevice struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	UniqueID string `json:"uniqueId"`
}

type traccarPosition struct {
	ID         int64                  `json:"id"`
	DeviceID   int64                  `json:"deviceId"`
	Protocol   string                 `json:"protocol"`
	Latitude   float64                `json:"latitude"`
	Longitude  float64                `json:"longitude"`
	Altitude   float64                `json:"altitude"`
	Speed      float64                `json:"speed"`
	Course     float64                `json:"course"`
	Valid      bool                   `json:"valid"`
	Attributes map[string]interface{} `json:"attributes"`
}

type traccarPayload struct {
	Device   traccarDevice   `json:"device"`
	Position traccarPosition `json:"position"`
}

type flatPayload struct {
	DeviceID     int64   `json:"deviceId"`
	UniqueID     string  `json:"uniqueId"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Altitude     float64 `json:"altitude"`
	Speed        float64 `json:"speed"`
	Course       float64 `json:"course"`
	Valid        bool    `json:"valid"`
	Protocol     string  `json:"protocol"`
	BatteryLevel int     `json:"batteryLevel"`
}

var (
	adapterURL     = getenv("ADAPTER_URL", "http://gps-adapter.railway.internal:8080/gps/position")
	forwardInvalid = getenv("FORWARD_INVALID", "false") == "true"
	client         = &http.Client{Timeout: 5 * time.Second}
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func batteryFromAttrs(a map[string]interface{}) int {
	v, ok := a["batteryLevel"]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		if n, err := strconv.Atoi(x); err == nil {
			return n
		}
	}
	return 0
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var p traccarPayload
	if err := json.Unmarshal(body, &p); err != nil {
		log.Printf("bad json: %v body=%s", err, truncate(body, 200))
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	if p.Device.ID == 0 || p.Position.DeviceID == 0 {
		log.Printf("skip: missing device id (device=%+v position.deviceId=%d)", p.Device, p.Position.DeviceID)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if !forwardInvalid && !p.Position.Valid {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	flat := flatPayload{
		DeviceID:     p.Position.DeviceID,
		UniqueID:     p.Device.UniqueID,
		Latitude:     p.Position.Latitude,
		Longitude:    p.Position.Longitude,
		Altitude:     p.Position.Altitude,
		Speed:        p.Position.Speed,
		Course:       p.Position.Course,
		Valid:        p.Position.Valid,
		Protocol:     p.Position.Protocol,
		BatteryLevel: batteryFromAttrs(p.Position.Attributes),
	}

	b, _ := json.Marshal(flat)
	resp, err := client.Post(adapterURL, "application/json", bytes.NewReader(b))
	if err != nil {
		log.Printf("adapter POST failed: deviceId=%d err=%v", flat.DeviceID, err)
		http.Error(w, "adapter unreachable", http.StatusBadGateway)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	log.Printf("fwd deviceId=%d name=%q lat=%.5f lon=%.5f valid=%t adapter=%d",
		flat.DeviceID, p.Device.Name, flat.Latitude, flat.Longitude, flat.Valid, resp.StatusCode)

	w.WriteHeader(http.StatusNoContent)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", handleWebhook)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	port := getenv("PORT", "8080")
	log.Printf("traccar-transformer listening on :%s adapter=%s forwardInvalid=%t",
		port, adapterURL, forwardInvalid)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
