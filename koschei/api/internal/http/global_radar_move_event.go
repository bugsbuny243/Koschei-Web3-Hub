package http

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/radarevent"
)

type globalRadarNetworkRequest struct {
	Network string `json:"network"`
}

func configuredMoveIdentityEndpoint(networkID string) string {
	switch strings.TrimSpace(networkID) {
	case "sui-mainnet":
		return strings.TrimSpace(os.Getenv("SUI_GRAPHQL_URL"))
	case "aptos-mainnet":
		return strings.TrimSpace(os.Getenv("APTOS_REST_URL"))
	default:
		return ""
	}
}

func decodeGlobalRadarNetworkRequest(reader io.Reader) (globalRadarNetworkRequest, error) {
	var result globalRadarNetworkRequest
	decoder := json.NewDecoder(reader)
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return result, io.ErrUnexpectedEOF
	}
	seen := false
	for decoder.More() {
		token, tokenErr := decoder.Token()
		key, ok := token.(string)
		if tokenErr != nil || !ok || key != "network" || seen {
			return result, io.ErrUnexpectedEOF
		}
		seen = true
		if decoder.Decode(&result.Network) != nil {
			return result, io.ErrUnexpectedEOF
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seen {
		return result, io.ErrUnexpectedEOF
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return result, io.ErrUnexpectedEOF
	}
	result.Network = strings.TrimSpace(result.Network)
	if result.Network == "" {
		return globalRadarNetworkRequest{}, io.ErrUnexpectedEOF
	}
	return result, nil
}

func globalRadarNetworkEvent(w http.ResponseWriter, r *http.Request) {
	globalRadarNetworkEventWithClient(w, r, nil)
}

func globalRadarNetworkEventWithClient(w http.ResponseWriter, r *http.Request, client *http.Client) {
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	defer r.Body.Close()

	reject := func(status int, message, availability string) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":              message,
			"analysis_performed": false,
			"evidence_status":    "unknown",
			"live_availability":  availability,
		})
	}

	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		reject(http.StatusUnsupportedMediaType, "json_required", "not_checked")
		return
	}
	request, err := decodeGlobalRadarNetworkRequest(r.Body)
	if err != nil {
		reject(http.StatusBadRequest, "invalid_network_request", "not_checked")
		return
	}
	network, ok := networktarget.LookupNetwork(request.Network)
	if !ok {
		reject(http.StatusUnprocessableEntity, "network_not_registered", "not_checked")
		return
	}
	if network.Family != "move" {
		reject(http.StatusUnprocessableEntity, "live_network_event_not_supported_for_network", "not_available")
		return
	}
	endpoint := configuredMoveIdentityEndpoint(network.ID)
	if endpoint == "" {
		reject(http.StatusServiceUnavailable, "move_identity_configuration_required", "configuration_required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	observedAt := time.Now().UTC()

	writeEvent := func(probe any, event radarevent.Event) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(globalRadarProbeEnvelope{
			SchemaVersion: globalRadarProbeEnvelopeSchema,
			SourceBinding: "normalized_probe_json_sha256",
			Probe:         probe,
			RadarEvent:    event,
		})
	}

	switch network.ID {
	case "sui-mainnet":
		result, probeErr := networktarget.ProbeSuiMainnetIdentity(ctx, client, endpoint, observedAt)
		if probeErr != nil {
			reject(http.StatusBadGateway, probeErr.Error(), "unavailable")
			return
		}
		sourceDigest, digestErr := normalizedProbeDigest(result)
		if digestErr != nil {
			reject(http.StatusInternalServerError, "radar_probe_digest_unavailable", "unavailable")
			return
		}
		event, eventErr := radarevent.BuildSuiIdentityEvent(
			"global-radar/sui-identity-probe-v1",
			result,
			sourceDigest,
		)
		if eventErr != nil {
			reject(http.StatusBadGateway, "radar_event_projection_unavailable", "unavailable")
			return
		}
		writeEvent(result, event)
	case "aptos-mainnet":
		result, probeErr := networktarget.ProbeAptosMainnetIdentity(ctx, client, endpoint, observedAt)
		if probeErr != nil {
			reject(http.StatusBadGateway, probeErr.Error(), "unavailable")
			return
		}
		sourceDigest, digestErr := normalizedProbeDigest(result)
		if digestErr != nil {
			reject(http.StatusInternalServerError, "radar_probe_digest_unavailable", "unavailable")
			return
		}
		event, eventErr := radarevent.BuildAptosIdentityEvent(
			"global-radar/aptos-identity-probe-v1",
			result,
			sourceDigest,
		)
		if eventErr != nil {
			reject(http.StatusBadGateway, "radar_event_projection_unavailable", "unavailable")
			return
		}
		writeEvent(result, event)
	default:
		reject(http.StatusUnprocessableEntity, "live_network_event_not_supported_for_network", "not_available")
	}
}
