// gps-adapter — Traccar → OpenRemote bridge.
//
// Accepts position pushes at POST /gps/position in EITHER flat shape
// (the legacy shape the old binary adapter understood) OR Traccar's native
// nested shape (`{device:{...}, position:{...}, event?:{...}}`) so Traccar's
// built-in forwarder can hit this endpoint directly.
//
// For every position received, the service:
//
//  1. Looks up the Traccar device (by deviceId) in an in-process cache.
//  2. Looks up the matching OpenRemote asset (by `traccarDeviceId` attribute)
//     also in cache, or queries OpenRemote if cache is cold.
//  3. Creates a new TrackerAsset under the configured Fleet parent if none
//     exists, or updates the existing one's attributes.
//
// Env (all optional unless marked required):
//
//	PORT                        default 8080
//	OPENREMOTE_URL              default http://manager.railway.internal:8080
//	OPENREMOTE_REALM            default unitip
//	OPENREMOTE_CLIENT_ID        default openremote
//	OPENREMOTE_USER             required (realm admin username)
//	OPENREMOTE_PASSWORD         required (realm admin password)
//	KEYCLOAK_URL                default http://keycloak.railway.internal:8080
//	TRACCAR_URL                 default http://traccar.railway.internal:8082
//	TRACCAR_USER                required (Traccar admin email)
//	TRACCAR_PASSWORD            required (Traccar admin password)
//	FLEET_PARENT_ID             optional; if set, new assets are created under it
//	FORWARD_INVALID             default false (skip positions with valid=false)
//
// Build: multi-stage Dockerfile in the same directory. Deploy via
// `railway up services/gps-adapter/src --path-as-root` after linking the
// GPS-Adapter service.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var cfg = loadConfig()

type config struct {
	port             string
	openremoteURL    string
	openremoteRealm  string
	openremoteClient string
	openremoteUser   string
	openremotePass   string
	keycloakURL      string
	traccarURL       string
	traccarUser      string
	traccarPass      string
	fleetParentID    string
	forwardInvalid   bool
}

func loadConfig() config {
	c := config{
		port:             env("PORT", "8080"),
		openremoteURL:    env("OPENREMOTE_URL", "http://manager.railway.internal:8080"),
		openremoteRealm:  env("OPENREMOTE_REALM", "unitip"),
		openremoteClient: env("OPENREMOTE_CLIENT_ID", "openremote"),
		openremoteUser:   env("OPENREMOTE_USER", ""),
		openremotePass:   env("OPENREMOTE_PASSWORD", ""),
		keycloakURL:      env("KEYCLOAK_URL", "http://keycloak.railway.internal:8080"),
		traccarURL:       env("TRACCAR_URL", "http://traccar.railway.internal:8082"),
		traccarUser:      env("TRACCAR_USER", ""),
		traccarPass:      env("TRACCAR_PASSWORD", ""),
		fleetParentID:    env("FLEET_PARENT_ID", ""),
		forwardInvalid:   env("FORWARD_INVALID", "false") == "true",
	}
	mustNonEmpty := map[string]string{
		"OPENREMOTE_USER":     c.openremoteUser,
		"OPENREMOTE_PASSWORD": c.openremotePass,
		"TRACCAR_USER":        c.traccarUser,
		"TRACCAR_PASSWORD":    c.traccarPass,
	}
	for k, v := range mustNonEmpty {
		if v == "" {
			log.Fatalf("variabila de mediu obligatorie lipsa: %s", k)
		}
	}
	return c
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ---------- Keycloak ----------

var tokenCache struct {
	sync.Mutex
	value   string
	expires time.Time
}

func getToken() (string, error) {
	tokenCache.Lock()
	defer tokenCache.Unlock()
	if tokenCache.value != "" && time.Now().Before(tokenCache.expires.Add(-10*time.Second)) {
		return tokenCache.value, nil
	}

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", cfg.openremoteClient)
	form.Set("username", cfg.openremoteUser)
	form.Set("password", cfg.openremotePass)

	url := fmt.Sprintf("%s/auth/realms/%s/protocol/openid-connect/token", cfg.keycloakURL, cfg.openremoteRealm)
	req, _ := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak HTTP %d: %s", resp.StatusCode, b)
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("keycloak decode: %w", err)
	}
	if tr.AccessToken == "" {
		return "", errors.New("token gol primit de la Keycloak")
	}
	tokenCache.value = tr.AccessToken
	tokenCache.expires = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return tokenCache.value, nil
}

// ---------- Traccar ----------

type traccarDevice struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	UniqueID string `json:"uniqueId"`
}

var deviceNameCache sync.Map // map[int64]traccarDevice

func getTraccarDevice(id int64) (traccarDevice, error) {
	if v, ok := deviceNameCache.Load(id); ok {
		return v.(traccarDevice), nil
	}
	reqURL := fmt.Sprintf("%s/api/devices/%d", cfg.traccarURL, id)
	req, _ := http.NewRequest("GET", reqURL, nil)
	req.SetBasicAuth(cfg.traccarUser, cfg.traccarPass)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return traccarDevice{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return traccarDevice{}, fmt.Errorf("traccar device lookup HTTP %d: %s", resp.StatusCode, b)
	}
	var d traccarDevice
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return traccarDevice{}, err
	}
	deviceNameCache.Store(id, d)
	return d, nil
}

// ---------- OpenRemote ----------

type attr struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value,omitempty"`
}

type asset struct {
	ID         string           `json:"id,omitempty"`
	Version    int64            `json:"version,omitempty"`
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	Realm      string           `json:"realm"`
	ParentID   string           `json:"parentId,omitempty"`
	Attributes map[string]*attr `json:"attributes"`
}

// assetCache: traccarDeviceId -> asset id in OpenRemote. Empty string means
// "we queried and there is no asset yet"; absence means "not yet queried".
var assetCache sync.Map // map[int64]string

// attrsReady: assetID -> true once the asset has been primed with the full
// set of TrackerAsset attributes via a full PUT. The batch attribute-event
// endpoint refuses unknown attribute names with INSUFFICIENT_ACCESS, so we
// have to ensure attributes exist on the asset before we can write to them
// via the fast path.
var attrsReady sync.Map // map[string]bool

var baselineAttrTypes = map[string]string{
	"altitude":     "number",
	"speed":        "number",
	"heading":      "number",
	"protocol":     "text",
	"batteryLevel": "positiveInteger",
	"fuelLevel":    "number",
	"ignition":     "boolean",
	"odometer":     "number",
}

func orURL(p string) string {
	return fmt.Sprintf("%s/api/%s/%s", cfg.openremoteURL, cfg.openremoteRealm, strings.TrimPrefix(p, "/"))
}

func orReq(method, path string, body any) ([]byte, int, error) {
	tok, err := getToken()
	if err != nil {
		return nil, 0, err
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, orURL(path), r)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	return buf, resp.StatusCode, nil
}

func preloadCache() {
	buf, code, err := orReq("POST", "/asset/query", map[string]any{
		"types": []string{"TrackerAsset"},
	})
	if err != nil || code != 200 {
		log.Printf("[cache] Preincarcare cache esuata: err=%v status=%d body=%s", err, code, snippet(buf))
		return
	}
	var assets []asset
	if err := json.Unmarshal(buf, &assets); err != nil {
		log.Printf("[cache] decode esuat: %v", err)
		return
	}
	n := 0
	for _, a := range assets {
		if v := a.Attributes["traccarDeviceId"]; v != nil {
			if tid, ok := toInt64(v.Value); ok {
				assetCache.Store(tid, a.ID)
				n++
			}
		}
	}
	log.Printf("[cache] %d assets incarcate in cache din OpenRemote realm=%s", n, cfg.openremoteRealm)
}

func findOrCreateAsset(tid int64, name string, uniqueID string) (string, error) {
	if v, ok := assetCache.Load(tid); ok && v.(string) != "" {
		return v.(string), nil
	}
	// Miss or negative cache — query OpenRemote
	buf, code, err := orReq("POST", "/asset/query", map[string]any{
		"types": []string{"TrackerAsset"},
		"attributes": map[string]any{
			"items": []map[string]any{{
				"name":  map[string]any{"predicateType": "string", "match": "EXACT", "value": "traccarDeviceId"},
				"value": map[string]any{"predicateType": "number", "operator": "EQUALS", "value": tid},
			}},
		},
	})
	if err != nil || code != 200 {
		return "", fmt.Errorf("asset/query HTTP %d: %s", code, snippet(buf))
	}
	var assets []asset
	if err := json.Unmarshal(buf, &assets); err != nil {
		return "", err
	}
	if len(assets) > 0 {
		assetCache.Store(tid, assets[0].ID)
		return assets[0].ID, nil
	}

	// Create new
	newAttrs := map[string]*attr{
		"traccarDeviceId": {Name: "traccarDeviceId", Type: "positiveInteger", Value: tid},
		"location":        {Name: "location", Type: "GEO_JSONPoint", Value: map[string]any{"type": "Point", "coordinates": []float64{26.1025, 44.4268}}},
	}
	if uniqueID != "" {
		newAttrs["serialNumber"] = &attr{Name: "serialNumber", Type: "text", Value: uniqueID}
	}
	newAsset := asset{
		Name:       name,
		Type:       "TrackerAsset",
		Realm:      cfg.openremoteRealm,
		ParentID:   cfg.fleetParentID,
		Attributes: newAttrs,
	}
	buf, code, err = orReq("POST", "/asset", newAsset)
	if err != nil || code != 200 {
		return "", fmt.Errorf("asset create HTTP %d: %s", code, snippet(buf))
	}
	var created asset
	if err := json.Unmarshal(buf, &created); err != nil {
		return "", err
	}
	assetCache.Store(tid, created.ID)
	log.Printf("[INFO] Asset nou creat in OpenRemote: %s (%s) pentru device Traccar %d", name, created.ID, tid)
	return created.ID, nil
}

// ensureAttributesExist does a one-off full asset PUT the first time we see
// an asset, adding any baseline attributes that are missing. After this, the
// batch AttributeEvent endpoint works for fast per-position updates.
func ensureAttributesExist(assetID string) error {
	if _, ok := attrsReady.Load(assetID); ok {
		return nil
	}
	buf, code, err := orReq("POST", "/asset/query", map[string]any{"ids": []string{assetID}})
	if err != nil || code != 200 {
		return fmt.Errorf("ensure: query HTTP %d: %s", code, snippet(buf))
	}
	var arr []asset
	if err := json.Unmarshal(buf, &arr); err != nil {
		return err
	}
	if len(arr) == 0 {
		return fmt.Errorf("ensure: asset %s not found", assetID)
	}
	a := arr[0]
	if a.Attributes == nil {
		a.Attributes = map[string]*attr{}
	}
	changed := false
	for name, typ := range baselineAttrTypes {
		if _, exists := a.Attributes[name]; !exists {
			a.Attributes[name] = &attr{Name: name, Type: typ}
			changed = true
		}
	}
	if changed {
		_, code, err := orReq("PUT", "/asset/"+assetID, a)
		if err != nil {
			return err
		}
		if code >= 300 {
			return fmt.Errorf("ensure: PUT asset HTTP %d", code)
		}
	}
	attrsReady.Store(assetID, true)
	return nil
}

// attributeEvent is the shape the bulk endpoint `PUT /asset/attributes` wants.
type attributeEvent struct {
	Ref struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"ref"`
	Value any `json:"value"`
}

// updateAttributes sends a batch of attribute events in one HTTP call.
// Requires that the attributes already exist on the asset — we guarantee
// that via ensureAttributesExist called once per asset lifetime.
func updateAttributes(assetID string, attrs map[string]any) error {
	if err := ensureAttributesExist(assetID); err != nil {
		return err
	}
	events := make([]attributeEvent, 0, len(attrs))
	for name, val := range attrs {
		if val == nil {
			continue
		}
		var ev attributeEvent
		ev.Ref.ID = assetID
		ev.Ref.Name = name
		ev.Value = val
		events = append(events, ev)
	}
	if len(events) == 0 {
		return nil
	}
	_, code, err := orReq("PUT", "/asset/attributes", events)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("PUT /asset/attributes HTTP %d", code)
	}
	return nil
}

// ---------- handlers ----------

type flatIncoming struct {
	DeviceID     int64   `json:"deviceId"`
	UniqueID     string  `json:"uniqueId"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Altitude     float64 `json:"altitude"`
	Speed        float64 `json:"speed"`
	Course       float64 `json:"course"`
	Valid        bool    `json:"valid"`
	Protocol     string  `json:"protocol"`
	BatteryLevel *int    `json:"batteryLevel,omitempty"`
}

type nestedIncoming struct {
	Device *struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		UniqueID string `json:"uniqueId"`
	} `json:"device,omitempty"`
	Position *struct {
		DeviceID   int64                  `json:"deviceId"`
		Latitude   float64                `json:"latitude"`
		Longitude  float64                `json:"longitude"`
		Altitude   float64                `json:"altitude"`
		Speed      float64                `json:"speed"`
		Course     float64                `json:"course"`
		Valid      bool                   `json:"valid"`
		Protocol   string                 `json:"protocol"`
		FixTime    string                 `json:"fixTime"`
		Attributes map[string]interface{} `json:"attributes"`
	} `json:"position,omitempty"`
}

// normalizePayload turns either a flat or a nested Traccar JSON body into a
// common struct the rest of the handler uses. Missing batteryLevel/fuel/etc.
// stays as nil so updateAttributes skips them instead of writing zeros.
type normalized struct {
	deviceID    int64
	uniqueID    string
	name        string
	lat, lon    float64
	altitude    float64
	speed       float64
	course      float64
	valid       bool
	protocol    string
	battery     *int
	fuel        *float64
	odometer    *float64
	ignition    *bool
	extraAttrs  map[string]interface{}
}

func parsePayload(body []byte) (*normalized, error) {
	// Try nested first (discriminator: presence of a non-empty "device" object)
	var n nestedIncoming
	if err := json.Unmarshal(body, &n); err == nil && n.Device != nil && n.Position != nil {
		out := &normalized{
			deviceID:   n.Device.ID,
			uniqueID:   n.Device.UniqueID,
			name:       n.Device.Name,
			lat:        n.Position.Latitude,
			lon:        n.Position.Longitude,
			altitude:   n.Position.Altitude,
			speed:      n.Position.Speed,
			course:     n.Position.Course,
			valid:      n.Position.Valid,
			protocol:   n.Position.Protocol,
			extraAttrs: n.Position.Attributes,
		}
		if out.deviceID == 0 {
			out.deviceID = n.Position.DeviceID
		}
		pullCommonAttrs(out, n.Position.Attributes)
		return out, nil
	}

	var f flatIncoming
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	out := &normalized{
		deviceID: f.DeviceID,
		uniqueID: f.UniqueID,
		lat:      f.Latitude, lon: f.Longitude,
		altitude: f.Altitude, speed: f.Speed, course: f.Course,
		valid: f.Valid, protocol: f.Protocol,
		battery: f.BatteryLevel,
	}
	return out, nil
}

func pullCommonAttrs(n *normalized, attrs map[string]interface{}) {
	if v, ok := attrs["batteryLevel"]; ok {
		if i, ok := toInt(v); ok {
			n.battery = &i
		}
	}
	if v, ok := attrs["fuelLevel"]; ok {
		if f, ok := toFloat(v); ok {
			n.fuel = &f
		}
	}
	for _, key := range []string{"odometer", "totalDistance"} {
		if v, ok := attrs[key]; ok {
			if f, ok := toFloat(v); ok {
				n.odometer = &f
				break
			}
		}
	}
	if v, ok := attrs["ignition"]; ok {
		if b, ok := v.(bool); ok {
			n.ignition = &b
		}
	}
}

func handlePosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	p, err := parsePayload(body)
	if err != nil {
		log.Printf("[WARN] payload invalid: %v body=%s", err, snippet(body))
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if p.deviceID == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !cfg.forwardInvalid && !p.valid {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Resolve device name if missing
	if p.name == "" {
		d, err := getTraccarDevice(p.deviceID)
		if err != nil {
			log.Printf("[WARN] traccar lookup deviceId=%d: %v", p.deviceID, err)
			p.name = fmt.Sprintf("Device %d", p.deviceID)
		} else {
			p.name = d.Name
			if p.uniqueID == "" {
				p.uniqueID = d.UniqueID
			}
		}
	}

	assetID, err := findOrCreateAsset(p.deviceID, p.name, p.uniqueID)
	if err != nil {
		log.Printf("[ERROR] getOrCreateAsset deviceId=%d: %v", p.deviceID, err)
		http.Error(w, "openremote unreachable", http.StatusBadGateway)
		return
	}

	attrs := map[string]any{
		"location":  map[string]any{"type": "Point", "coordinates": []float64{p.lon, p.lat}},
		"altitude":  p.altitude,
		"speed":     p.speed,
		"heading":   p.course,
		"protocol":  p.protocol,
	}
	if p.battery != nil {
		attrs["batteryLevel"] = *p.battery
	}
	if p.fuel != nil {
		attrs["fuelLevel"] = *p.fuel
	}
	if p.odometer != nil {
		attrs["odometer"] = *p.odometer
	}
	if p.ignition != nil {
		attrs["ignition"] = *p.ignition
	}

	if err := updateAttributes(assetID, attrs); err != nil {
		log.Printf("[ERROR] updateAttributes asset=%s: %v", assetID, err)
		http.Error(w, "openremote write failed", http.StatusBadGateway)
		return
	}

	log.Printf("[OK] device=%d asset=%s lat=%.5f lon=%.5f speed=%.1fkm/h",
		p.deviceID, assetID, p.lat, p.lon, p.speed)

	w.WriteHeader(http.StatusNoContent)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, `{"status":"ok","time":%q}`, time.Now().UTC().Format(time.RFC3339Nano))
}

// ---------- helpers ----------

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int:
		return int64(x), true
	case int64:
		return x, true
	case string:
		if n, err := strconv.ParseInt(x, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

func toInt(v any) (int, bool) {
	n, ok := toInt64(v)
	return int(n), ok
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func snippet(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}

// ---------- main ----------

func main() {
	http.DefaultClient.Timeout = 10 * time.Second
	log.Printf("GPS Adapter pornit pe :%s realm=%s openremote=%s fleet=%q", cfg.port, cfg.openremoteRealm, cfg.openremoteURL, cfg.fleetParentID)

	go preloadCache()

	mux := http.NewServeMux()
	mux.HandleFunc("/gps/position", handlePosition)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	log.Fatal(http.ListenAndServe(":"+cfg.port, mux))
}
