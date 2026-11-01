package services

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func testAuthorityWatchTransaction(signature, watch string, slot int64, inner bool) SolanaTransactionResult {
	instruction := map[string]any{
		"programId": splTokenProgramID,
		"parsed": map[string]any{
			"type":"setAuthority",
			"info":map[string]any{"account":watch,"authorityType":"mintTokens",
				"newAuthority":defaultPumpProgramID},
		},
	}
	outer := []any{instruction}
	metaInner := []any{}
	if inner {
		outer = []any{map[string]any{"programId":defaultRaydiumProgramID,
			"parsed":map[string]any{"type":"swap","info":map[string]any{}}}}
		metaInner = []any{map[string]any{"index":float64(0),
			"instructions":[]any{instruction}}}
	}
	return SolanaTransactionResult{
		"slot":float64(slot),
		"transaction":map[string]any{
			"signatures":[]any{signature},
			"message":map[string]any{"instructions":outer},
		},
		"meta":map[string]any{"err":nil,"innerInstructions":metaInner},
	}
}

func TestAuthorityWatchConfigurationFailClosed(t *testing.T) {
	watch:=defaultRaydiumProgramID
	for _,raw:=range []string{"", "invalid", watch+","+watch, watch+","+watch+","+watch+","+watch} {
		if _,err:=solanaAuthorityWatchAddresses(raw);err==nil {
			t.Fatalf("accepted unsafe watch list: %q",raw)
		}
	}
	got,err:=solanaAuthorityWatchAddresses(" "+watch+" ")
	if err!=nil || len(got)!=1 || got[0]!=watch {
		t.Fatalf("valid exact watch list failed: %v %v",got,err)
	}
	t.Setenv("KOSCHEI_SOLANA_AUTHORITY_WATCH_ENABLED","")
	if envBool("KOSCHEI_SOLANA_AUTHORITY_WATCH_ENABLED") {
		t.Fatal("watch must be off by default")
	}
}

func TestAuthorityWatchVerifiesExactConfirmedTransactionAndCPI(t *testing.T) {
	watch:=defaultRaydiumProgramID
	sig:=strings.Repeat("3",87)
	tx:=testAuthorityWatchTransaction(sig,watch,123456,false)
	findings,err:=analyzeConfirmedSolanaAuthorityWatch(watch,sig,123456,tx)
	if err!=nil || len(findings)!=1 || findings[0].Scope!="outer" ||
		findings[0].Severity!="critical" || findings[0].AuthorityType!="mintTokens" ||
		len(findings[0].TransactionSHA256)!=64 {
		t.Fatalf("missing verified authority change: %+v %v",findings,err)
	}
	cpi:=testAuthorityWatchTransaction(sig,watch,123456,true)
	found,err:=analyzeConfirmedSolanaAuthorityWatch(watch,sig,123456,cpi)
	if err!=nil || len(found)!=1 || found[0].Scope!="cpi" ||
		found[0].InnerIndex!=1 || found[0].InstructionIndex!=0 {
		t.Fatalf("nested CPI authority change lost: %+v %v",found,err)
	}
	other,err:=analyzeConfirmedSolanaAuthorityWatch(defaultPumpSwapProgramID,sig,123456,tx)
	if err!=nil || len(other)!=0 {t.Fatalf("other account triggered false positive: %+v %v",other,err)}
}

func TestAuthorityWatchMissingOrForgedNativeProofWithholds(t *testing.T) {
	watch:=defaultRaydiumProgramID
	sig:=strings.Repeat("4",87)
	for _,tc:=range []struct {
		name string
		alter func(SolanaTransactionResult)
	}{
		{"wrong_signature",func(tx SolanaTransactionResult){tx["transaction"].(map[string]any)["signatures"]=[]any{"not-original"}}},
		{"wrong_slot",func(tx SolanaTransactionResult){tx["slot"]=float64(999)}},
		{"failed_transaction",func(tx SolanaTransactionResult){tx["meta"].(map[string]any)["err"]=map[string]any{"InstructionError":[]any{0,"Custom"}}}},
		{"missing_err",func(tx SolanaTransactionResult){delete(tx["meta"].(map[string]any),"err")}},
		{"missing_inner",func(tx SolanaTransactionResult){delete(tx["meta"].(map[string]any),"innerInstructions")}},
		{"corrupt_outer",func(tx SolanaTransactionResult){tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"]="not-an-array"}},
		{"unparsed_token",func(tx SolanaTransactionResult){
			tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"].([]any)[0].(map[string]any)["parsed"]=nil
		}},
		{"missing_new_authority_type",func(tx SolanaTransactionResult){
			ins:=tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"].([]any)[0].(map[string]any)
			delete(ins["parsed"].(map[string]any)["info"].(map[string]any),"authorityType")
		}},
		{"malformed_cpi",func(tx SolanaTransactionResult){
			tx["meta"].(map[string]any)["innerInstructions"]=[]any{map[string]any{"index":float64(50),"instructions":[]any{}}}
		}},
	}{
		t.Run(tc.name,func(t *testing.T){
			tx:=testAuthorityWatchTransaction(sig,watch,123456,false)
			tc.alter(tx)
			if findings,err:=analyzeConfirmedSolanaAuthorityWatch(watch,sig,123456,tx);err==nil {
				t.Fatalf("untrusted RPC data was accepted: %+v",findings)
			}
		})
	}
}

func TestAuthorityWatchBenignTransferAndRevocationAreNotAttacks(t *testing.T) {
	watch:=defaultRaydiumProgramID
	sig:=strings.Repeat("5",87)
	tx:=testAuthorityWatchTransaction(sig,watch,123456,false)
	entry:=tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"].([]any)[0].(map[string]any)
	entry["parsed"]=map[string]any{"type":"transferChecked",
		"info":map[string]any{"account":watch}}
	events,err:=analyzeConfirmedSolanaAuthorityWatch(watch,sig,123456,tx)
	if err!=nil || len(events)!=0 { t.Fatalf("normal token transfer misclassified: %v %+v",err,events) }
	entry["parsed"]=map[string]any{"type":"setAuthority",
		"info":map[string]any{"account":watch,"authorityType":"accountOwner","newAuthority":nil}}
	events,err=analyzeConfirmedSolanaAuthorityWatch(watch,sig,123456,tx)
	if err!=nil || len(events)!=1 || events[0].NewAuthority!="revoked" || events[0].Severity!="medium" {
		t.Fatalf("revocation was confused with hostile grant: %v %+v",err,events)
	}
}

func TestAuthorityWatchPostgresAtomicCursorAndGap(t *testing.T) {
	dsn:=os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn=="" {t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")}
	db,err:=sql.Open("postgres",dsn)
	if err!=nil {t.Fatal(err)}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx,cancel:=context.WithTimeout(context.Background(),35*time.Second)
	defer cancel()
	if err:=db.PingContext(ctx);err!=nil {t.Fatal(err)}
	for _,stmt:=range []string{
		"CREATE TEMP TABLE security_solana_authority_watch_cursors (LIKE public.security_solana_authority_watch_cursors INCLUDING ALL)",
		"CREATE TEMP TABLE security_solana_authority_watch_observations (LIKE public.security_solana_authority_watch_observations INCLUDING ALL)",
	}{
		if _,err:=db.ExecContext(ctx,stmt);err!=nil{t.Fatal(err)}
	}
	watch:=defaultRaydiumProgramID
	older:=strings.Repeat("3",87)
	newer:=strings.Repeat("4",87)
	signatures:=[]SolanaSignatureInfo{{Signature:newer,Slot:123457},{Signature:older,Slot:123456}}
	worker:=&solanaAuthorityWatchSensor{DB:db,RPCURL:"isolated-fixture",
		FetchSignatures:func(_ context.Context,address,before string,limit int)([]SolanaSignatureInfo,error){
			if address!=watch || limit!=solanaAuthorityWatchLimit {t.Fatalf("unexpected watch RPC params: %q %q %d",address,before,limit)}
			return signatures,nil
		},
		FetchTransaction:func(_ context.Context,sig string)(SolanaTransactionResult,error){
			tx:=testAuthorityWatchTransaction(sig,watch,123456,false)
			if sig==newer {tx["slot"]=float64(123457)}
			if sig==older {
				tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"]=[]any{map[string]any{"programId":defaultPumpProgramID}}
			}
			return tx,nil
		},
	}
	if err:=worker.processAddress(ctx,watch);err!=nil {t.Fatal(err)}
	assertState:=func(count int, watermark,status string) {
		t.Helper()
		var recorded int
		if err:=db.QueryRowContext(ctx,"SELECT count(*) FROM security_solana_authority_watch_observations").Scan(&recorded);err!=nil {t.Fatal(err)}
		var gotSig,gotStatus string
		if err:=db.QueryRowContext(ctx,"SELECT watermark_signature,status FROM security_solana_authority_watch_cursors WHERE watch_address=$1",watch).Scan(&gotSig,&gotStatus);err!=nil{t.Fatal(err)}
		if recorded!=count || gotSig!=watermark || gotStatus!=status {
			t.Fatalf("false evidence or cursor: count=%d sig=%s status=%s",recorded,gotSig,gotStatus)
		}
	}
	assertState(1,newer,"caught_up")
	if err:=worker.processAddress(ctx,watch);err!=nil{t.Fatal(err)}
	assertState(1,newer,"caught_up")
	// Next page cannot reach its previous watermark; refuse to advance.
	signatures=[]SolanaSignatureInfo{{Signature:strings.Repeat("6",87),Slot:123460}}
	if err:=worker.processAddress(ctx,watch);err==nil {t.Fatal("missing historical watermark accepted")}
	assertState(1,newer,"gap_unresolved")
	// Restore the watermark but make native transaction unavailable: no
	// inserts or checkpoint movement may occur.
	signatures=[]SolanaSignatureInfo{{Signature:strings.Repeat("7",87),Slot:123461},{Signature:newer,Slot:123457}}
	worker.FetchTransaction=func(_ context.Context,_ string)(SolanaTransactionResult,error){
		return nil,errors.New("provider unavailable")
	}
	if err:=worker.processAddress(ctx,watch);err==nil{t.Fatal("RPC failure advanced cursor")}
	assertState(1,newer,"rpc_error")
}
