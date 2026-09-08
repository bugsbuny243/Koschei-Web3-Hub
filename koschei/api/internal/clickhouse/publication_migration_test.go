package clickhouse

import "testing"

func TestValidateTrustedPublicationMigration(t *testing.T) {
	valid := `
CREATE TABLE IF NOT EXISTS koschei_web3.dossier_bundle_manifests
(
    manifest_id UUID
)
ENGINE = MergeTree
ORDER BY manifest_id;

CREATE TABLE IF NOT EXISTS koschei_web3.dossier_publication_transitions
(
    transition_id UUID
)
ENGINE = MergeTree
ORDER BY transition_id;
`
	if err := validateTrustedPublicationMigration(valid); err != nil {
		t.Fatalf("valid publication migration rejected: %v", err)
	}

	for name, candidate := range map[string]string{
		"drop":      valid + "\nDROP TABLE koschei_web3.dossier_bundle_manifests;",
		"unrelated": `CREATE TABLE IF NOT EXISTS koschei_web3.customer_secrets (id UInt64) ENGINE=MergeTree ORDER BY id;`,
		"missing":   `CREATE TABLE IF NOT EXISTS koschei_web3.dossier_bundle_manifests (manifest_id UUID) ENGINE=MergeTree ORDER BY manifest_id;`,
		"duplicate": valid + `\nCREATE TABLE IF NOT EXISTS koschei_web3.dossier_bundle_manifests (manifest_id UUID) ENGINE=MergeTree ORDER BY manifest_id;`,
		"alter":     `ALTER TABLE koschei_web3.dossier_bundle_manifests ADD COLUMN secret String;`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateTrustedPublicationMigration(candidate); err == nil {
				t.Fatal("unsafe publication migration unexpectedly accepted")
			}
		})
	}
}
