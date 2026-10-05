package cryptobrief

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"koschei/api/internal/workerwake"
)

// ARVISInbound handles only account pairing and ARVIS result-delivery controls.
// It intentionally has no news/RSS commands.
func (s *Service) ARVISInbound(ctx context.Context, event, recipient, text string) error {
	if event == "" || len(event) > 256 || !digits.MatchString(recipient) || len(text) > 512 {
		return errors.New("inbound_invalid")
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}
	command := strings.ToLower(strings.Split(fields[0], "@")[0])
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `INSERT INTO crypto_brief_inbound(channel,event_id) VALUES('telegram',$1) ON CONFLICT DO NOTHING`, event)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return nil
	}

	var id int64
	var state, customer string
	reply := ""
	if command == "/start" && len(fields) == 2 {
		if len(fields[1]) != 32 {
			return ErrPairInvalid
		}
		err = tx.QueryRowContext(ctx, `SELECT customer_sub FROM crypto_brief_pairings WHERE channel='telegram' AND token_hash=$1 AND expires_at>clock_timestamp() FOR UPDATE`, hash(fields[1])).Scan(&customer)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPairInvalid
		}
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `UPDATE crypto_brief_subscriptions SET recipient=$2,state='active',consent_at=clock_timestamp(),last_inbound_at=clock_timestamp(),updated_at=clock_timestamp() WHERE customer_sub=$1 AND channel='telegram' AND state='disconnected' RETURNING id,state`, customer, recipient).Scan(&id, &state)
		if err != nil {
			return ErrPairInvalid
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM crypto_brief_pairings WHERE customer_sub=$1 AND channel='telegram'`, customer); err != nil {
			return err
		}
		reply = "🛡️ ARVIS Telegram bağlantısı hazır.\nKoschei'de yaptığınız ARVIS taramalarının sonuçları burada da gönderilecek.\n/son: son tarama · /dur: bildirimleri durdur · /devam: aç · /sil: bağlantıyı kaldır\nHesap: https://tradepigloball.co/account#arvis-telegram"
	} else {
		err = tx.QueryRowContext(ctx, `SELECT id,state,customer_sub FROM crypto_brief_subscriptions WHERE channel='telegram' AND recipient=$1 FOR UPDATE`, recipient).Scan(&id, &state, &customer)
		if errors.Is(err, sql.ErrNoRows) {
			return tx.Commit()
		}
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET last_inbound_at=clock_timestamp() WHERE id=$1`, id); err != nil {
			return err
		}

		switch command {
		case "/dur", "dur", "stop", "/stop":
			state = "paused"
		case "/devam", "devam":
			state = "active"
			reply = "ARVIS Telegram bildirimleri açıldı."
		case "/sil", "sil", "delete", "/delete":
			state = "disconnected"
		case "/son", "son":
			if state == "active" {
				reply = s.latestARVISResult(ctx, tx, customer)
				if reply == "" {
					reply = "Henüz tamamlanmış bir ARVIS taramanız yok. Tarama: https://tradepigloball.co/scan"
				}
			}
		default:
			if state == "active" {
				reply = "ARVIS sonuç botu. Tarama tamamlandığında sonuç otomatik gelir.\n/son · /dur · /devam · /sil\nTarama: https://tradepigloball.co/scan"
			}
		}

		if state == "disconnected" {
			_, err = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET state='disconnected',recipient=NULL,consent_at=NULL,last_inbound_at=NULL,updated_at=clock_timestamp() WHERE id=$1`, id)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET state=$2,updated_at=clock_timestamp() WHERE id=$1`, id, state)
		}
		if err != nil {
			return err
		}
		if state != "active" {
			if _, err = tx.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET state='cancelled',body='ARVIS Telegram bildirimi iptal edildi.',reason='customer_opt_out',updated_at=clock_timestamp() WHERE subscription_id=$1 AND state='pending'`, id); err != nil {
				return err
			}
		}
		if state == "disconnected" {
			if _, err = tx.ExecContext(ctx, `DELETE FROM crypto_brief_pairings WHERE customer_sub=$1 AND channel='telegram'`, customer); err != nil {
				return err
			}
		}
	}

	if reply != "" && state == "active" {
		_, err = tx.ExecContext(ctx, `INSERT INTO crypto_brief_deliveries(subscription_id,dedup_key,body) SELECT $1,$2,$3 WHERE NOT EXISTS(SELECT 1 FROM crypto_brief_deliveries WHERE subscription_id=$1 AND dedup_key LIKE 'reply:%' AND created_at>clock_timestamp()-interval '1 minute') ON CONFLICT DO NOTHING`, id, "reply:"+event, reply)
		if err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	workerwake.Signal(wakeID)
	return nil
}

func (s *Service) latestARVISResult(ctx context.Context, tx *sql.Tx, customer string) string {
	var target, network string
	var payload []byte
	err := tx.QueryRowContext(ctx, `
		SELECT target,network,result_payload
		FROM web3_jobs
		WHERE user_id=$1 AND job_type='canonical_investigation' AND status='completed' AND result_payload IS NOT NULL
		ORDER BY completed_at DESC NULLS LAST,updated_at DESC
		LIMIT 1`, customer).Scan(&target, &network, &payload)
	if err != nil {
		return ""
	}
	var envelope map[string]any
	if json.Unmarshal(payload, &envelope) != nil {
		return ""
	}
	return FormatARVISResult(target, network, envelope)
}
