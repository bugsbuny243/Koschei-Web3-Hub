package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const (
	solanaAuthorityWatchLimit        = 10
	solanaAuthorityWatchMaxPages     = 2
	solanaAuthorityWatchMaxAddresses = 3
)

type solanaAuthorityWatchFinding struct {
	WatchAddress      string
	Signature         string
	Slot              int64
	Scope             string
	InstructionIndex  int
	InnerIndex        int
	ProgramID         string
	Account           string
	AuthorityType     string
	NewAuthority      string
	Severity          string
	TransactionSHA256 string
}

type solanaAuthorityWatchCursor struct {
	Signature string
	Slot      int64
}

type solanaAuthorityWatchSensor struct {
	DB        *sql.DB
	RPCURL    string
	Addresses []string
	Interval  time.Duration
	// Only tests replace these transport functions. Production uses the
	// existing budgeted, SSRF-restricted Solana JSON-RPC client.
	FetchSignatures  func(context.Context, string, string, int) ([]SolanaSignatureInfo, error)
	FetchTransaction func(context.Context, string) (SolanaTransactionResult, error)
}

func solanaAuthorityWatchAddresses(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("authority watch addresses were not explicitly configured")
	}
	parts := strings.Split(raw, ",")
	if len(parts) > solanaAuthorityWatchMaxAddresses {
		return nil, errors.New("authority watch address limit exceeded")
	}
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		address := strings.TrimSpace(part)
		if !isLikelyRadarSolanaAddress(address) || seen[address] {
			return nil, errors.New("invalid or repeated Solana authority watch address")
		}
		out = append(out, address)
		seen[address] = true
	}
	return out, nil
}

// Explicit opt-in only; never piggyback on the expensive global WSS switches.
func StartSolanaAuthorityWatchSensorIfEnabled(ctx context.Context, db *sql.DB) func() {
	if db == nil || !envBool("KOSCHEI_SOLANA_AUTHORITY_WATCH_ENABLED") {
		return func() {}
	}
	addresses, err := solanaAuthorityWatchAddresses(os.Getenv("KOSCHEI_SOLANA_AUTHORITY_WATCH_ADDRESSES"))
	if err != nil {
		log.Printf("Solana authority watch withheld: invalid watch configuration")
		return func() {}
	}
	rpcURL := resolveSecurityRadarRPCURL()
	if strings.TrimSpace(rpcURL) == "" {
		log.Printf("Solana authority watch withheld: trusted RPC provider unavailable")
		return func() {}
	}
	seconds := boundedSecurityRadarEnvInt("KOSCHEI_SOLANA_AUTHORITY_WATCH_INTERVAL_SECONDS", 120, 60, 900)
	child, cancel := context.WithCancel(ctx)
	worker := &solanaAuthorityWatchSensor{
		DB: db, RPCURL: rpcURL, Addresses: addresses,
		Interval: time.Duration(seconds) * time.Second,
	}
	go worker.start(child)
	return cancel
}

func (w *solanaAuthorityWatchSensor) start(ctx context.Context) {
	log.Printf("Solana authority watch started watch_count=%d interval=%s coverage=opt_in_confirmed_only", len(w.Addresses), w.Interval)
	w.runOnce(ctx)
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *solanaAuthorityWatchSensor) runOnce(ctx context.Context) {
	for _, address := range w.Addresses {
		if ctx.Err() != nil {
			return
		}
		cycle, cancel := context.WithTimeout(ctx, 45*time.Second)
		err := w.processAddress(cycle, address)
		cancel()
		if err != nil && ctx.Err() == nil {
			// Never log signed URLs or raw RPC errors (which may contain API keys).
			log.Printf("Solana authority watch cycle withheld address=%s reason=coverage_or_evidence_unavailable", address)
		}
	}
}

func (w *solanaAuthorityWatchSensor) signatures(ctx context.Context, address, before string) ([]SolanaSignatureInfo, error) {
	if w.FetchSignatures != nil {
		return w.FetchSignatures(ctx, address, before, solanaAuthorityWatchLimit)
	}
	config := map[string]any{"limit": solanaAuthorityWatchLimit, "commitment": "confirmed"}
	if before != "" {
		config["before"] = before
	}
	return solanaRPCDo[[]SolanaSignatureInfo](ctx, w.RPCURL, "getSignaturesForAddress", []any{address, config})
}

func (w *solanaAuthorityWatchSensor) transaction(ctx context.Context, signature string) (SolanaTransactionResult, error) {
	if w.FetchTransaction != nil {
		return w.FetchTransaction(ctx, signature)
	}
	return SolanaGetTransactionJSONParsed(ctx, w.RPCURL, signature)
}

func (w *solanaAuthorityWatchSensor) cursor(ctx context.Context, address string) (solanaAuthorityWatchCursor, error) {
	var value solanaAuthorityWatchCursor
	err := w.DB.QueryRowContext(ctx, `SELECT watermark_signature,watermark_slot
		FROM security_solana_authority_watch_cursors WHERE watch_address=$1`, address).
		Scan(&value.Signature, &value.Slot)
	if errors.Is(err, sql.ErrNoRows) {
		return value, nil
	}
	return value, err
}

func (w *solanaAuthorityWatchSensor) markProblem(ctx context.Context, address, status string) {
	_, _ = w.DB.ExecContext(ctx, `
		INSERT INTO security_solana_authority_watch_cursors
		(watch_address,status,last_error,last_polled_at,updated_at)
		VALUES ($1,$2,'coverage_or_evidence_unavailable',now(),now())
		ON CONFLICT (watch_address) DO UPDATE
		SET status=$2,last_error='coverage_or_evidence_unavailable',
		    last_polled_at=now(),updated_at=now()
	`, address, status)
}

// processAddress never advances a watermark past a failed getTransaction,
// incomplete CPI body, DB write failure, or an RPC pagination gap.
func (w *solanaAuthorityWatchSensor) processAddress(ctx context.Context, address string) error {
	if w == nil || w.DB == nil || !isLikelyRadarSolanaAddress(address) {
		return errors.New("authority watch storage or address unavailable")
	}
	cursor, err := w.cursor(ctx, address)
	if err != nil {
		return err
	}
	collected := make([]SolanaSignatureInfo, 0, solanaAuthorityWatchLimit*solanaAuthorityWatchMaxPages)
	seen := make(map[string]bool)
	before := ""
	reached := cursor.Signature == ""
	exhausted := false
	bootstrapTruncated := false
	for page := 0; page < solanaAuthorityWatchMaxPages; page++ {
		signatures, fetchErr := w.signatures(ctx, address, before)
		if fetchErr != nil {
			w.markProblem(ctx, address, "rpc_error")
			return fetchErr
		}
		if len(signatures) > solanaAuthorityWatchLimit {
			w.markProblem(ctx, address, "rpc_error")
			return errors.New("provider signature page exceeded bound")
		}
		if cursor.Signature == "" && page == 0 {
			bootstrapTruncated = len(signatures) == solanaAuthorityWatchLimit
		}
		for _, item := range signatures {
			if !securityRadarValidSignature(item.Signature) || item.Slot <= 0 || seen[item.Signature] {
				w.markProblem(ctx, address, "rpc_error")
				return errors.New("untrusted Solana signature pagination")
			}
			seen[item.Signature] = true
			if item.Signature == cursor.Signature {
				reached = true
				break
			}
			collected = append(collected, item)
		}
		if reached || len(signatures) < solanaAuthorityWatchLimit {
			exhausted = len(signatures) < solanaAuthorityWatchLimit
			break
		}
		before = signatures[len(signatures)-1].Signature
	}
	if cursor.Signature != "" && !reached {
		w.markProblem(ctx, address, "gap_unresolved")
		return errors.New("prior watermark was not observed within bounded pagination")
	}
	if len(collected) == 0 {
		if cursor.Signature == "" && exhausted {
			return nil
		}
		_, _ = w.DB.ExecContext(ctx, `UPDATE security_solana_authority_watch_cursors
			SET last_polled_at=now(),updated_at=now() WHERE watch_address=$1`, address)
		return nil
	}
	// Bootstrap deliberately inspects ONLY the most recent page. Historical
	// coverage is explicitly incomplete and never labelled caught-up.
	if cursor.Signature == "" && len(collected) > solanaAuthorityWatchLimit {
		collected = collected[:solanaAuthorityWatchLimit]
	}
	findings := make([]solanaAuthorityWatchFinding, 0)
	for i := len(collected) - 1; i >= 0; i-- {
		item := collected[i]
		if item.Err != nil {
			continue // failed transaction cannot mutate state; cursor still advances
		}
		tx, fetchErr := w.transaction(ctx, item.Signature)
		if fetchErr != nil {
			w.markProblem(ctx, address, "rpc_error")
			return fetchErr
		}
		events, decodeErr := analyzeConfirmedSolanaAuthorityWatch(address, item.Signature, item.Slot, tx)
		if decodeErr != nil {
			w.markProblem(ctx, address, "rpc_error")
			return decodeErr
		}
		findings = append(findings, events...)
	}
	// Atomic insert + cursor advance; duplicate sensor instances cannot race
	// a stale cursor forward without observing an optimistic-lock mismatch.
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO security_solana_authority_watch_cursors
		(watch_address) VALUES ($1) ON CONFLICT DO NOTHING`, address)
	if err != nil {
		return err
	}
	var actual solanaAuthorityWatchCursor
	if err = tx.QueryRowContext(ctx, `SELECT watermark_signature,watermark_slot
		FROM security_solana_authority_watch_cursors WHERE watch_address=$1 FOR UPDATE`, address).
		Scan(&actual.Signature, &actual.Slot); err != nil {
		return err
	}
	if actual != cursor {
		return errors.New("watch cursor moved during remote fetch; retry without a false checkpoint")
	}
	for _, event := range findings {
		_, err = tx.ExecContext(ctx, `INSERT INTO security_solana_authority_watch_observations
			(watch_address,signature,slot,instruction_scope,instruction_index,inner_index,
			 program_id,authority_account,authority_type,new_authority,severity,transaction_sha256)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (watch_address,signature,instruction_scope,instruction_index,inner_index) DO NOTHING`,
			event.WatchAddress, event.Signature, event.Slot, event.Scope, event.InstructionIndex,
			event.InnerIndex, event.ProgramID, event.Account, event.AuthorityType,
			event.NewAuthority, event.Severity, event.TransactionSHA256)
		if err != nil {
			return err
		}
		// Verified *exposure*, not maliciousness. A high/critical observation
		// becomes a deduped owner SOC review candidate in the same atomic
		// transaction as its source proof and cursor. A delivery failure can
		// never silently lose the incident candidate after checkpoint advance.
		if event.Severity == "high" || event.Severity == "critical" {
			payload, marshalErr := json.Marshal(map[string]any{
				"schema_version":     "koschei.solana-authority-watch.v1",
				"playbook_id":        "SOC-PB-02",
				"network":            "solana-mainnet",
				"signature":          event.Signature,
				"slot":               event.Slot,
				"instruction_scope":  event.Scope,
				"instruction_index":  event.InstructionIndex,
				"inner_index":        event.InnerIndex,
				"program_id":         event.ProgramID,
				"authority_type":     event.AuthorityType,
				"new_authority":      event.NewAuthority,
				"transaction_sha256": event.TransactionSHA256,
				"evidence_state":     "confirmed_executed_transaction",
				"enforcement_state":  "not_connected",
				"report_integrity":   "unsigned_rpc_observation",
			})
			if marshalErr != nil {
				return marshalErr
			}
			key := fmt.Sprintf("solana-authority-watch:%x", sha256.Sum256([]byte(
				event.WatchAddress+"|"+event.Signature+"|"+event.Scope+"|"+
					fmt.Sprintf("%d|%d", event.InstructionIndex, event.InnerIndex))))
			_, err = tx.ExecContext(ctx, `INSERT INTO security_alert_events
				(source,event_type,severity,target,title,message,dedupe_key,payload)
				VALUES ('solana_authority_watch','security.alert.created',$1,$2,
				'Observed Solana token authority change — owner review required',
				'Confirmed onchain token authority mutation. This is an exposure signal, not proof of unauthorized access or malicious intent.',
				$3,$4::jsonb)
				ON CONFLICT (dedupe_key) DO NOTHING`,
				event.Severity, event.WatchAddress, key, string(payload))
			if err != nil {
				return err
			}
		}
	}
	status := "caught_up"
	if cursor.Signature == "" && bootstrapTruncated {
		status = "bootstrap_window_only"
	}
	head := collected[0]
	_, err = tx.ExecContext(ctx, `UPDATE security_solana_authority_watch_cursors
		SET watermark_signature=$2,watermark_slot=$3,status=$4,last_error='',
		    last_polled_at=now(),updated_at=now() WHERE watch_address=$1`,
		address, head.Signature, head.Slot, status)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// analyzeConfirmedSolanaAuthorityWatch accepts neither logs nor asserted
// opcode labels as proof; it requires exact getTransaction signature/slot,
// success metadata, and decoded SPL instructions including any inner CPI.
func analyzeConfirmedSolanaAuthorityWatch(watch, signature string, slot int64, tx SolanaTransactionResult) ([]solanaAuthorityWatchFinding, error) {
	meta := asRadarMap(tx["meta"])
	transaction := asRadarMap(tx["transaction"])
	signatures, ok := transaction["signatures"].([]any)
	if !ok || len(signatures) == 0 || anyString(signatures[0]) != signature ||
		radarInt64(tx["slot"]) != slot || slot <= 0 || !securityRadarValidSignature(signature) {
		return nil, errors.New("transaction identity was not confirmed")
	}
	if _, exists := meta["err"]; !exists || meta["err"] != nil {
		return nil, errors.New("transaction execution is not confirmed successful")
	}
	message := asRadarMap(transaction["message"])
	outer, ok := message["instructions"].([]any)
	if !ok {
		return nil, errors.New("transaction outer instructions missing")
	}
	if _, exists := meta["innerInstructions"]; !exists {
		return nil, errors.New("transaction CPI evidence unavailable")
	}
	inner := []any{}
	if meta["innerInstructions"] != nil {
		var cast bool
		inner, cast = meta["innerInstructions"].([]any)
		if !cast {
			return nil, errors.New("transaction inner CPI evidence malformed")
		}
	}
	encoded, err := json.Marshal(tx)
	if err != nil || len(encoded) > 2<<20 {
		return nil, errors.New("transaction evidence exceeded digest budget")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(encoded))
	found := []solanaAuthorityWatchFinding{}
	appendInstruction := func(raw any, scope string, i, j int) error {
		entry := asRadarMap(raw)
		program := anyString(entry["programId"])
		if program != splTokenProgramID && program != token2022ProgramID {
			return nil
		}
		parsed := asRadarMap(entry["parsed"])
		if len(parsed) == 0 {
			return errors.New("unparsed SPL token instruction; cannot assert complete authority coverage")
		}
		kind := anyString(parsed["type"])
		if kind != "setAuthority" && kind != "initializePermanentDelegate" {
			return nil
		}
		info := asRadarMap(parsed["info"])
		account := strings.TrimSpace(anyString(info["account"]))
		if account == "" {
			account = strings.TrimSpace(anyString(info["mint"]))
		}
		if account == "" || !isLikelyRadarSolanaAddress(account) {
			return errors.New("authority instruction account identity unavailable")
		}
		if account != watch {
			return nil // only exact configured account; not every token transaction
		}
		authorityType := anyString(info["authorityType"])
		newAuthority := anyString(info["newAuthority"])
		if kind == "initializePermanentDelegate" {
			if program != token2022ProgramID {
				return errors.New("permanent delegate from untrusted program")
			}
			authorityType = "permanentDelegate"
			newAuthority = anyString(info["delegate"])
		}
		if authorityType == "" {
			return errors.New("SPL authority mutation type unavailable")
		}
		switch authorityType {
		case "mintTokens", "freezeAccount", "accountOwner", "closeAccount", "permanentDelegate":
		default:
			return errors.New("unsupported SPL authority mutation type")
		}
		severity := "high"
		if newAuthority == "" {
			newAuthority = "revoked"
			severity = "medium"
		} else if !isLikelyRadarSolanaAddress(newAuthority) {
			return errors.New("new authority was malformed")
		} else if authorityType == "mintTokens" || authorityType == "freezeAccount" || authorityType == "permanentDelegate" {
			severity = "critical"
		}
		found = append(found, solanaAuthorityWatchFinding{
			WatchAddress: watch, Signature: signature, Slot: slot, Scope: scope,
			InstructionIndex: i, InnerIndex: j, ProgramID: program, Account: account,
			AuthorityType: authorityType, NewAuthority: newAuthority,
			Severity: severity, TransactionSHA256: digest,
		})
		if len(found) > 32 {
			return errors.New("authority instruction count exceeds safe bound")
		}
		return nil
	}
	for i, v := range outer {
		if err := appendInstruction(v, "outer", i, 0); err != nil {
			return nil, err
		}
	}
	for _, rawGroup := range inner {
		group := asRadarMap(rawGroup)
		index := int(radarInt64(group["index"]))
		if index < 0 || index >= len(outer) {
			return nil, errors.New("invalid CPI instruction group index")
		}
		entries, ok := group["instructions"].([]any)
		if !ok {
			return nil, errors.New("CPI instruction group has missing entries")
		}
		for j, v := range entries {
			if err := appendInstruction(v, "cpi", index, j+1); err != nil {
				return nil, err
			}
		}
	}
	return found, nil
}
