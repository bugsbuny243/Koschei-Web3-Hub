package defense

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

func PersistRuntimeReport(ctx context.Context, db *sql.DB, report RuntimeReport) (RuntimeReport, error) {
	if db == nil {
		SetPersistenceStatus(&report, "database_unavailable")
		return report, errors.New("defense runtime database is unavailable")
	}
	stored := report
	SetPersistenceStatus(&stored, "persisted")
	payload, err := json.Marshal(stored)
	if err != nil {
		SetPersistenceStatus(&report, "marshal_failed")
		return report, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		SetPersistenceStatus(&report, "begin_failed")
		return report, err
	}
	defer func() { _ = tx.Rollback() }()

	var runID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO defense_agent_runs
		(case_ref,target,network,execution_mode,runtime_version,status,verdict_authority,input_hash,report_hash,report_json,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,false,$7,$8,$9::jsonb,$10)
		ON CONFLICT (case_ref) DO NOTHING
		RETURNING id::text`,
		stored.CaseRef, stored.Target, stored.Network, stored.ExecutionMode, stored.SchemaVersion,
		stored.Status, stored.InputHash, stored.ReportHash, string(payload), stored.GeneratedAt,
	).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT id::text FROM defense_agent_runs WHERE case_ref=$1`, stored.CaseRef).Scan(&runID)
	}
	if err != nil {
		SetPersistenceStatus(&report, "run_insert_failed")
		return report, err
	}

	for _, invocation := range stored.ToolInvocations {
		inputJSON, marshalErr := json.Marshal(invocation.Input)
		if marshalErr != nil {
			SetPersistenceStatus(&report, "tool_input_marshal_failed")
			return report, marshalErr
		}
		outputJSON, marshalErr := json.Marshal(invocation.Output)
		if marshalErr != nil {
			SetPersistenceStatus(&report, "tool_output_marshal_failed")
			return report, marshalErr
		}
		evidenceJSON, marshalErr := json.Marshal(invocation.EvidenceIDs)
		if marshalErr != nil {
			SetPersistenceStatus(&report, "tool_evidence_marshal_failed")
			return report, marshalErr
		}
		limitationsJSON, marshalErr := json.Marshal(invocation.Limitations)
		if marshalErr != nil {
			SetPersistenceStatus(&report, "tool_limitations_marshal_failed")
			return report, marshalErr
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO defense_tool_invocations
			(agent_run_id,tool_run_id,agent_role,tool_name,status,input_hash,output_hash,input_json,output_json,evidence_refs,limitations,started_at,finished_at)
			VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,$11::jsonb,$12,$13)
			ON CONFLICT (tool_run_id) DO NOTHING`,
			runID, invocation.ToolRunID, invocation.AgentRole, invocation.ToolName, invocation.Status,
			invocation.InputHash, invocation.OutputHash, string(inputJSON), string(outputJSON),
			string(evidenceJSON), string(limitationsJSON), invocation.StartedAt, invocation.FinishedAt,
		)
		if err != nil {
			SetPersistenceStatus(&report, "tool_insert_failed")
			return report, err
		}
	}
	if err = tx.Commit(); err != nil {
		SetPersistenceStatus(&report, "commit_failed")
		return report, err
	}
	return stored, nil
}
