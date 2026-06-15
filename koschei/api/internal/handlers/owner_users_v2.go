package handlers

import (
	"database/sql"
	"net/http"
	"strings"
)

func (h *Handler) OwnerUsersV2(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "owner schema unavailable"})
		return
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	authSource := `SELECT NULL::text AS email, NULL::text AS auth_subject, NULL::timestamptz AS created_at, NULL::timestamptz AS updated_at WHERE false`
	if ownerTableExists(r.Context(), h.DB, "local_auth_users") {
		authSource = `
			SELECT lower(email) AS email,
			       COALESCE(auth_subject,'') AS auth_subject,
			       created_at,
			       updated_at
			FROM local_auth_users
			WHERE COALESCE(email,'') <> ''`
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		WITH local_users AS (`+authSource+`),
		local_ranked AS (
			SELECT *, row_number() OVER (PARTITION BY lower(email) ORDER BY COALESCE(updated_at, created_at, now()) DESC) rn
			FROM local_users
		),
		profile_ranked AS (
			SELECT p.*, row_number() OVER (
				PARTITION BY lower(p.email)
				ORDER BY CASE COALESCE(p.status,'active') WHEN 'active' THEN 0 WHEN 'banned' THEN 1 ELSE 2 END,
				         COALESCE(p.updated_at, p.created_at, now()) DESC
			) rn
			FROM app_user_profiles p
			WHERE COALESCE(p.email,'') <> ''
		),
		people AS (
			SELECT email FROM local_ranked WHERE rn=1
			UNION
			SELECT lower(email) FROM profile_ranked WHERE rn=1
		)
		SELECT
			COALESCE(p.id::text, 'local:' || md5(pe.email), 'email:' || md5(pe.email)) AS id,
			COALESCE(NULLIF(p.auth_subject,''), NULLIF(l.auth_subject,''), 'local:' || pe.email) AS auth_subject,
			pe.email,
			COALESCE(p.wallet_address,'') AS wallet_address,
			COALESCE(p.credits,0) AS credits,
			CASE
				WHEN COALESCE(p.status,'') = 'banned' THEN 'banned'
				WHEN COALESCE(p.status,'') = 'removed' AND l.email IS NOT NULL THEN 'active'
				ELSE COALESCE(NULLIF(p.status,''),'active')
			END AS status,
			COALESCE(p.created_at, l.created_at, now()) AS created_at,
			COALESCE(p.updated_at, l.updated_at, p.created_at, l.created_at, now()) AS updated_at,
			p.banned_at,
			ent.plan_id,
			COALESCE(ent.status, 'No active package') AS entitlement_status,
			ent.expires_at
		FROM people pe
		LEFT JOIN local_ranked l ON lower(l.email)=lower(pe.email) AND l.rn=1
		LEFT JOIN profile_ranked p ON lower(p.email)=lower(pe.email) AND p.rn=1
		LEFT JOIN LATERAL (
			SELECT CASE COALESCE(e.plan_id,'') WHEN 'builder' THEN 'professional' WHEN 'studio' THEN 'enterprise' ELSE COALESCE(e.plan_id,'') END AS plan_id,
			       e.status,
			       e.expires_at
			FROM entitlements e
			WHERE lower(e.email)=lower(pe.email)
			  AND e.status='active'
			  AND COALESCE(e.plan_id,'') <> ''
			  AND COALESCE(e.plan_id,'') <> 'free'
			  AND (e.expires_at IS NULL OR e.expires_at > now())
			ORDER BY e.updated_at DESC, e.created_at DESC
			LIMIT 1
		) ent ON true
		WHERE ($1 = '' OR lower(pe.email) LIKE $2 OR lower(COALESCE(p.wallet_address,'')) LIKE $2 OR lower(COALESCE(p.auth_subject,l.auth_subject,'')) LIKE $2)
		ORDER BY COALESCE(l.created_at, p.created_at, now()) DESC
		LIMIT 500`, q, "%"+q+"%")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed", "message": err.Error()})
		return
	}
	defer rows.Close()

	users := []ownerUserRecord{}
	for rows.Next() {
		var u ownerUserRecord
		var planID sql.NullString
		var entitlementStatus string
		var expiresAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.AuthSubject, &u.Email, &u.WalletAddress, &u.Credits, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.BannedAt, &planID, &entitlementStatus, &expiresAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed", "message": err.Error()})
			return
		}
		u.LegacyCredits = u.Credits
		if planID.Valid && strings.TrimSpace(planID.String) != "" {
			plan := normalizePackageID(planID.String)
			if plan == "" {
				plan = strings.TrimSpace(planID.String)
			}
			u.PlanID = &plan
		}
		u.ActiveEntitlementStatus = entitlementStatus
		u.EntitlementExpiresAt = nullTimePtr(expiresAt)
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "users": users, "source": "local_auth_users+app_user_profiles+entitlements"})
}
