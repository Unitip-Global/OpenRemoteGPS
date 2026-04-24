// traccar-transformer — passthrough proxy in front of gps-adapter.
//
// Traccar's built-in forwarder posts a nested JSON payload
// ({device:{...}, position:{...}}) to this service. We parse only enough
// to enforce the FORWARD_INVALID filter and to log per-message activity,
// then forward the original request body unchanged so every attribute
// (bleTemp1, power, ignition, ioXX, etc.) reaches gps-adapter, which now
// accepts the nested shape natively.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
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
	Latitude   float64                `json:"latitude"`
	Longitude  float64                `json:"longitude"`
	Valid      bool                   `json:"valid"`
	Attributes map[string]interface{} `json:"attributes"`
}

type traccarPayload struct {
	Device   traccarDevice   `json:"device"`
	Position traccarPosition `json:"position"`
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

	deviceID := p.Position.DeviceID
	if deviceID == 0 {
		deviceID = p.Device.ID
	}
	if deviceID == 0 {
		log.Printf("skip: missing device id (device=%+v position.deviceId=%d)", p.Device, p.Position.DeviceID)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// NOTE: we no longer drop invalid positions here — the adapter handles
	// that nuance (it keeps telemetry attrs like BLE temperature / ignition
	// /  battery but skips overwriting location when valid=false). Dropping
	// at the transformer threw away perfectly good sensor data from devices
	// without a current GPS fix (indoor, sat=0 but BLE beacons still
	// reporting).
	_ = forwardInvalid

	resp, err := client.Post(adapterURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("adapter POST failed: deviceId=%d err=%v", deviceID, err)
		http.Error(w, "adapter unreachable", http.StatusBadGateway)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	log.Printf("fwd deviceId=%d name=%q lat=%.5f lon=%.5f valid=%t attrs=%d adapter=%d",
		deviceID, p.Device.Name, p.Position.Latitude, p.Position.Longitude, p.Position.Valid, len(p.Position.Attributes), resp.StatusCode)

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
