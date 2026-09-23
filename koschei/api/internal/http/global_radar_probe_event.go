package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime"
	"net/http"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/radarevent"
)

const globalRadarProbeEnvelopeSchema = "koschei.global-radar-probe-event.v1"

type globalRadarProbeEnvelope struct {
	SchemaVersion string           `json:"schema_version"`
	SourceBinding string           `json:"source_binding"`
	Probe         any              `json:"probe"`
	RadarEvent    radarevent.Event `json:"radar_event"`
}

func globalRadarProbeEvent(w http.ResponseWriter, r *http.Request) {
	globalRadarProbeEventWithClient(w, r, nil)
}

func globalRadarProbeEventWithClient(w http.ResponseWriter, r *http.Request, client *http.Client) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
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
	request, err := decodeNetworkTargetRequest(r.Body)
	if err != nil {
		reject(http.StatusBadRequest, "invalid_target_request", "not_checked")
		return
	}
	resolution, err := networktarget.Resolve(request.Network, request.Address)
	if err != nil {
		reject(http.StatusUnprocessableEntity, err.Error(), "not_checked")
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

	switch resolution.Network.Family {
	case "evm":
		endpoint := configuredEVMRPCEndpoint(resolution.Network.ID)
		if endpoint == "" {
			reject(http.StatusServiceUnavailable, "evm_rpc_configuration_required", "configuration_required")
			return
		}
		result, probeErr := networktarget.ProbeEVM(ctx, client, endpoint, resolution)
		if probeErr != nil {
			reject(http.StatusBadGateway, probeErr.Error(), "unavailable")
			return
		}
		sourceDigest, digestErr := normalizedProbeDigest(result)
		if digestErr != nil {
			reject(http.StatusInternalServerError, "radar_probe_digest_unavailable", "unavailable")
			return
		}
		event, eventErr := radarevent.BuildEVMAddressProbeEvent(
			"global-radar/evm-address-probe-v1:"+resolution.Network.ID,
			result,
			observedAt,
			sourceDigest,
		)
		if eventErr != nil {
			reject(http.StatusBadGateway, "radar_event_projection_unavailable", "unavailable")
			return
		}
		writeEvent(result, event)
	case "utxo":
		if resolution.Network.ID != "bitcoin-mainnet" {
			reject(http.StatusUnprocessableEntity, "live_radar_event_not_supported_for_network", "not_available")
			return
		}
		endpoint := configuredBitcoinEsploraEndpoint()
		if endpoint == "" {
			reject(http.StatusServiceUnavailable, "bitcoin_esplora_configuration_required", "configuration_required")
			return
		}
		result, probeErr := networktarget.ProbeBitcoin(ctx, client, endpoint, resolution)
		if probeErr != nil {
			reject(http.StatusBadGateway, probeErr.Error(), "unavailable")
			return
		}
		sourceDigest, digestErr := normalizedProbeDigest(result)
		if digestErr != nil {
			reject(http.StatusInternalServerError, "radar_probe_digest_unavailable", "unavailable")
			return
		}
		event, eventErr := radarevent.BuildBitcoinAddressProbeEvent(
			"global-radar/bitcoin-address-probe-v1",
			result,
			observedAt,
			sourceDigest,
		)
		if eventErr != nil {
			reject(http.StatusBadGateway, "radar_event_projection_unavailable", "unavailable")
			return
		}
		writeEvent(result, event)
	default:
		reject(http.StatusUnprocessableEntity, "live_radar_event_not_supported_for_network", "not_available")
	}
}

func normalizedProbeDigest(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
