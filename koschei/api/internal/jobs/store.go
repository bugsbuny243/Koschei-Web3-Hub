package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Store struct{ DB *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{DB: db} }

func (s *Store) Create(ctx context.Context, in CreateInput) (Job, error) {
	if s == nil || s.DB == nil {
		return Job{}, errors.New("job store database unavailable")
	}
	payload, err := json.Marshal(in.Request)
	if err != nil {
		return Job{}, err
	}
	var j Job
	err = s.DB.QueryRowContext(ctx, `INSERT INTO web3_jobs (user_id,email,job_type,status,network,target,request_payload,progress) VALUES ($1,$2,$3,'queued',$4,$5,$6,0) RETURNING id,user_id,email,job_type,status,network,target,request_payload,COALESCE(result_payload,'null'::jsonb),COALESCE(error_code,''),COALESCE(error_message,''),progress,attempts,queued_at,updated_at`, in.UserID, in.Email, in.Type, in.Network, in.Target, payload).Scan(&j.ID, &j.UserID, &j.Email, &j.Type, &j.Status, &j.Network, &j.Target, &j.RequestPayload, &j.ResultPayload, &j.ErrorCode, &j.ErrorMessage, &j.Progress, &j.Attempts, &j.QueuedAt, &j.UpdatedAt)
	return j, err
}
func (s *Store) Get(ctx context.Context, id, userID string) (Job, error) {
	var j Job
	q := `SELECT id,user_id,email,job_type,status,network,target,request_payload,COALESCE(result_payload,'null'::jsonb),COALESCE(error_code,''),COALESCE(error_message,''),progress,attempts,queued_at,updated_at FROM web3_jobs WHERE id=$1`
	args := []any{id}
	if userID != "" {
		q += ` AND user_id=$2`
		args = append(args, userID)
	}
	err := s.DB.QueryRowContext(ctx, q, args...).Scan(&j.ID, &j.UserID, &j.Email, &j.Type, &j.Status, &j.Network, &j.Target, &j.RequestPayload, &j.ResultPayload, &j.ErrorCode, &j.ErrorMessage, &j.Progress, &j.Attempts, &j.QueuedAt, &j.UpdatedAt)
	return j, err
}
func (s *Store) MarkRunning(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE web3_jobs SET status='running',progress=10,attempts=attempts+1,started_at=COALESCE(started_at,now()),updated_at=now() WHERE id=$1`, id)
	return err
}
func (s *Store) Complete(ctx context.Context, id string, result any) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE web3_jobs SET status='completed',progress=100,result_payload=$2,error_code=NULL,error_message=NULL,completed_at=now(),updated_at=now() WHERE id=$1`, id, b)
	return err
}
func (s *Store) Fail(ctx context.Context, id, code, message string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE web3_jobs SET status='failed',error_code=$2,error_message=$3,failed_at=now(),updated_at=now() WHERE id=$1`, id, code, message)
	return err
}
