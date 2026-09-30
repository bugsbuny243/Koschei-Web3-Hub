package runtimeinstance

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/runtimehealth"
)

const (
	HealthID          = "runtime.instance-heartbeat"
	SchemaVersion     = "koschei.runtime-instance-heartbeat.v1"
	HeartbeatInterval = 30 * time.Second
	StaleAfter        = 90 * time.Second
	writeTimeout      = 5 * time.Second
	stopTimeout       = 5 * time.Second
)

type Metadata struct {
	InstanceID               string
	RuntimeRole              string
	ServiceName              string
	EnvironmentName          string
	DeploymentID             string
	ReplicaID                string
	Region                   string
	HTTPEnabled              bool
	BackgroundWorkersEnabled bool
	StartedAt                time.Time
}

type Runner struct {
	db       *sql.DB
	meta     Metadata
	health   *runtimehealth.Registry
	interval time.Duration
	now      func() time.Time

	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func New(db *sql.DB, role string, httpEnabled, backgroundWorkersEnabled bool, health *runtimehealth.Registry) *Runner {
	now := time.Now().UTC()
	return &Runner{
		db:       db,
		meta:     metadataFromEnvironment(role, httpEnabled, backgroundWorkersEnabled, now),
		health:   health,
		interval: HeartbeatInterval,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (r *Runner) Metadata() Metadata {
	if r == nil {
		return Metadata{}
	}
	return r.meta
}

func (r *Runner) Start(parent context.Context) func() {
	if r == nil {
		return func() {}
	}
	configured := r.db != nil
	if r.health != nil {
		r.health.Register(HealthID, "runtime", "", configured)
	}
	if !configured {
		return func() {}
	}

	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.done = make(chan struct{})
	go r.loop(ctx)

	return func() { r.Stop() }
}

func (r *Runner) Stop() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		if r.done != nil {
			select {
			case <-r.done:
			case <-time.After(stopTimeout):
			}
		}
		r.markStopped(context.Background())
		if r.health != nil {
			r.health.Stop(HealthID)
		}
	})
}

func (r *Runner) loop(ctx context.Context) {
	defer close(r.done)
	r.writeHeartbeat(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.writeHeartbeat(ctx)
		}
	}
}

func (r *Runner) writeHeartbeat(parent context.Context) {
	if r == nil || r.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, writeTimeout)
	defer cancel()
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO runtime_instance_heartbeats (
			instance_id,runtime_role,service_name,environment_name,deployment_id,replica_id,region,
			http_enabled,background_workers_enabled,started_at,heartbeat_at,stopped_at,updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULL,$11)
		ON CONFLICT (instance_id) DO UPDATE
		SET runtime_role=EXCLUDED.runtime_role,
		    service_name=EXCLUDED.service_name,
		    environment_name=EXCLUDED.environment_name,
		    deployment_id=EXCLUDED.deployment_id,
		    replica_id=EXCLUDED.replica_id,
		    region=EXCLUDED.region,
		    http_enabled=EXCLUDED.http_enabled,
		    background_workers_enabled=EXCLUDED.background_workers_enabled,
		    heartbeat_at=EXCLUDED.heartbeat_at,
		    stopped_at=NULL,
		    updated_at=EXCLUDED.updated_at
	`,
		r.meta.InstanceID, r.meta.RuntimeRole, r.meta.ServiceName, r.meta.EnvironmentName,
		r.meta.DeploymentID, r.meta.ReplicaID, r.meta.Region,
		r.meta.HTTPEnabled, r.meta.BackgroundWorkersEnabled, r.meta.StartedAt, now,
	)
	if err != nil {
		log.Printf("runtime heartbeat write failed instance=%s: %v", r.meta.InstanceID, err)
		if r.health != nil {
			r.health.Failure(HealthID, err)
		}
		return
	}
	if r.health != nil {
		r.health.Success(HealthID, 1)
	}
}

func (r *Runner) markStopped(parent context.Context) {
	if r == nil || r.db == nil || strings.TrimSpace(r.meta.InstanceID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(parent, writeTimeout)
	defer cancel()
	now := r.now().UTC()
	if _, err := r.db.ExecContext(ctx, `
		UPDATE runtime_instance_heartbeats
		SET stopped_at=$2, heartbeat_at=$2, updated_at=$2
		WHERE instance_id=$1
	`, r.meta.InstanceID, now); err != nil {
		log.Printf("runtime heartbeat stop marker failed instance=%s: %v", r.meta.InstanceID, err)
	}
}

func metadataFromEnvironment(role string, httpEnabled, backgroundWorkersEnabled bool, startedAt time.Time) Metadata {
	deploymentID := boundedEnv("RAILWAY_DEPLOYMENT_ID", 256)
	replicaID := boundedEnv("RAILWAY_REPLICA_ID", 256)
	explicitID := boundedEnv("KOSCHEI_RUNTIME_INSTANCE_ID", 256)
	instanceID := explicitID
	if instanceID == "" {
		nonce := randomNonce()
		switch {
		case deploymentID != "" && replicaID != "":
			instanceID = "railway:" + deploymentID + ":" + replicaID + ":" + nonce
		case replicaID != "":
			instanceID = "railway:" + replicaID + ":" + nonce
		default:
			instanceID = "runtime:" + nonce
		}
	}
	return Metadata{
		InstanceID:               instanceID,
		RuntimeRole:              strings.ToLower(strings.TrimSpace(role)),
		ServiceName:              boundedEnv("RAILWAY_SERVICE_NAME", 256),
		EnvironmentName:          boundedEnv("RAILWAY_ENVIRONMENT_NAME", 256),
		DeploymentID:             deploymentID,
		ReplicaID:                replicaID,
		Region:                   boundedEnv("RAILWAY_REPLICA_REGION", 128),
		HTTPEnabled:              httpEnabled,
		BackgroundWorkersEnabled: backgroundWorkersEnabled,
		StartedAt:                startedAt.UTC(),
	}
}

func boundedEnv(name string, max int) string {
	value := strings.TrimSpace(os.Getenv(name))
	if max > 0 && len(value) > max {
		value = value[:max]
	}
	return value
}

func randomNonce() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}
