package services

import (
    "context"
    "database/sql"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "strings"
    "sync/atomic"
    "testing"
    "time"
    _ "github.com/lib/pq"
)

func pb02WatchFixture(watch, sig string, slot int64, authority bool, cpi bool) SolanaTransactionResult {
    outer:=[]any{map[string]any{"programId":"11111111111111111111111111111111","parsed":map[string]any{"type":"transfer"}}}
    event:=map[string]any{"programId":socPB02TokenProgram,"parsed":map[string]any{
        "type":"setAuthority","info":map[string]any{
            "account":watch,"authorityType":"freezeAccount","newAuthority":strings.Repeat("7",44),
        },
    }}
    if authority&&!cpi {outer=append(outer,event)}
    inner:=[]any{}
    if authority&&cpi {inner=append(inner,map[string]any{"index":float64(0),"instructions":[]any{event}})}
    return SolanaTransactionResult{
        "slot":float64(slot),
        "meta":map[string]any{"err":nil,"innerInstructions":inner},
        "transaction":map[string]any{
            "signatures":[]any{sig},
            "message":map[string]any{
                "accountKeys":[]any{map[string]any{"pubkey":watch,"signer":false}},
                "instructions":outer,
            },
        },
    }
}

func TestPB02BoundedWatchConfigurationAndEvidence(t *testing.T) {
    watch:=strings.Repeat("8",44)
    signature:=strings.Repeat("3",87)
    if _,err:=parseSOCWatchAddresses("");err==nil{t.Fatal("empty watch must fail closed")}
    if _,err:=parseSOCWatchAddresses(strings.Join([]string{watch,watch},","));err==nil{t.Fatal("duplicate watch must be rejected")}
    if _,err:=parseSOCWatchAddresses(strings.Join([]string{watch,"abc"},","));err==nil{t.Fatal("bad address accepted")}
    if _,err:=parseSOCWatchAddresses(strings.Join([]string{watch,strings.Repeat("9",44),strings.Repeat("A",44),strings.Repeat("B",44),strings.Repeat("C",44)},","));err==nil{t.Fatal("unbounded watch accepted")}
    if addresses,err:=parseSOCWatchAddresses(watch);err!=nil||len(addresses)!=1{t.Fatalf("one address: %v %+v",err,addresses)}

    for _,inner:=range []bool{false,true} {
        tx:=pb02WatchFixture(watch,signature,101,true,inner)
        events,err:=inspectSOCpb02ParsedTransaction(watch,SolanaSignatureInfo{Signature:signature,Slot:101},tx)
        if err!=nil||len(events)!=1||events[0].Severity!="critical" {
            t.Fatalf("watch missed authority change (inner=%v): events=%+v err=%v",inner,events,err)
        }
        want:="outer"
        if inner {want="inner"}
        if events[0].Source!=want{t.Fatalf("wrong CPI origin: %+v",events[0])}
    }
    normal:=pb02WatchFixture(watch,signature,101,false,false)
    result,err:=inspectSOCpb02ParsedTransaction(watch,SolanaSignatureInfo{Signature:signature,Slot:101},normal)
    if err!=nil||len(result)!=0{t.Fatalf("normal transaction flagged: %+v %v",result,err)}
    for _,tc:=range []struct{
        name string
        mutate func(SolanaTransactionResult)
    }{
        {"wrong_slot",func(tx SolanaTransactionResult){tx["slot"]=float64(102)}},
        {"wrong_signature",func(tx SolanaTransactionResult){asRadarMap(tx["transaction"])["signatures"]=[]any{strings.Repeat("4",87)}}},
        {"missing_inner_cpi",func(tx SolanaTransactionResult){delete(asRadarMap(tx["meta"]),"innerInstructions")}},
        {"unknown_meta",func(tx SolanaTransactionResult){delete(asRadarMap(tx["meta"]),"err")}},
        {"wrong_watch",func(tx SolanaTransactionResult){asRadarMap(asRadarMap(tx["transaction"])["message"])["accountKeys"]=[]any{map[string]any{"pubkey":strings.Repeat("A",44)}}}},
        {"malformed_token_program_instruction",func(tx SolanaTransactionResult){
            asRadarMap(asRadarMap(tx["transaction"])["message"])["instructions"]=[]any{map[string]any{"programId":socPB02TokenProgram}}
        }},
        {"invalid_inner_parent",func(tx SolanaTransactionResult){
            asRadarMap(tx["meta"])["innerInstructions"]=[]any{map[string]any{"index":float64(999),"instructions":[]any{}}}
        }},
    }{
        t.Run(tc.name,func(t *testing.T){
            tx:=pb02WatchFixture(watch,signature,101,true,true)
            tc.mutate(tx)
            if events,err:=inspectSOCpb02ParsedTransaction(watch,SolanaSignatureInfo{Signature:signature,Slot:101},tx);err==nil {
                t.Fatalf("missing or forged evidence must withhold, got %+v",events)
            }
        })
    }
}

func TestPB02BoundedWatchPostgresRPCAndCursorRecovery(t *testing.T) {
    dsn:=os.Getenv("KOSCHEI_TEST_DATABASE_URL")
    if dsn==""{t.Skip("KOSCHEI_TEST_DATABASE_URL unavailable")}
    t.Setenv("APP_ENV","test")
    t.Setenv("SOLANA_RPC_BUDGET_ENABLED","false")
    t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS","0")
    db,err:=sql.Open("postgres",dsn)
    if err!=nil{t.Fatal(err)}
    defer db.Close()
    db.SetMaxOpenConns(1)
    ctx,cancel:=context.WithTimeout(context.Background(),35*time.Second)
    defer cancel()
    if err:=db.PingContext(ctx);err!=nil{t.Fatal(err)}
    for _,table:=range []string{"security_soc_pb02_watch_cursors","security_soc_pb02_authority_observations"}{
        if _,err:=db.ExecContext(ctx,"CREATE TEMP TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)");err!=nil{t.Fatal(err)}
    }
    watch:=strings.Repeat("8",44)
    sig1:=strings.Repeat("3",87)
    sig2:=strings.Repeat("4",87)
    sig3:=strings.Repeat("5",87)
    var phase atomic.Int32
    var calls atomic.Int32
    server:=httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        var request struct{
            Method string `json:"method"`
            Params []any `json:"params"`
        }
        if err:=json.NewDecoder(r.Body).Decode(&request);err!=nil{t.Errorf("RPC decode: %v",err);w.WriteHeader(400);return}
        calls.Add(1)
        var value any
        switch request.Method{
        case "getSignaturesForAddress":
            switch phase.Load(){
            case 0:value=[]any{map[string]any{"signature":sig1,"slot":101,"err":nil}}
            case 1:value=[]any{map[string]any{"signature":sig2,"slot":102,"err":nil},map[string]any{"signature":sig1,"slot":101,"err":nil}}
            default:value=[]any{map[string]any{"signature":sig3,"slot":103,"err":nil},map[string]any{"signature":sig2,"slot":102,"err":nil},map[string]any{"signature":sig1,"slot":101,"err":nil}}
            }
        case "getTransaction":
            sig,_:=request.Params[0].(string)
            switch sig{
            case sig2:value=pb02WatchFixture(watch,sig2,102,true,true)
            case sig3:
                value=pb02WatchFixture(watch,sig3,103,true,false)
                delete(asRadarMap(value.(SolanaTransactionResult)["meta"]),"innerInstructions")
            default:t.Errorf("unwanted transaction lookup for %s",sig);w.WriteHeader(400);return
            }
        default:t.Errorf("unwanted method %s",request.Method);w.WriteHeader(400);return
        }
        w.Header().Set("Content-Type","application/json")
        _=json.NewEncoder(w).Encode(map[string]any{"jsonrpc":"2.0","id":1,"result":value})
    }))
    defer server.Close()
    previous:=solanaRPCClient
    solanaRPCClient=server.Client()
    defer func(){solanaRPCClient=previous}()
    worker:=&socPB02AuthorityWatch{DB:db,RPCURL:server.URL,Addresses:[]string{watch}}
    if err:=worker.pollAddress(ctx,watch);err!=nil{t.Fatalf("first-start cursor bootstrap: %v",err)}
    var cursor string
    if err:=db.QueryRowContext(ctx,"SELECT watermark_signature FROM security_soc_pb02_watch_cursors").Scan(&cursor);err!=nil||cursor!=sig1{
        t.Fatalf("bootstrap missing: %q %v",cursor,err)
    }
    phase.Store(1)
    if err:=worker.pollAddress(ctx,watch);err!=nil{t.Fatalf("real RPC authority observation: %v",err)}
    var count int
    if err:=db.QueryRowContext(ctx,"SELECT count(*) FROM security_soc_pb02_authority_observations").Scan(&count);err!=nil||count!=1{
        t.Fatalf("authority change was not durable: count=%d err=%v",count,err)
    }
    if err:=worker.pollAddress(ctx,watch);err!=nil{t.Fatalf("idempotent repeat: %v",err)}
    if err:=db.QueryRowContext(ctx,"SELECT count(*) FROM security_soc_pb02_authority_observations").Scan(&count);err!=nil||count!=1 {
        t.Fatalf("duplicate observation written: count=%d err=%v",count,err)
    }
    phase.Store(2)
    if err:=worker.pollAddress(ctx,watch);err==nil{t.Fatal("missing inner CPI evidence must halt cursor")}
    var latest string
    if err:=db.QueryRowContext(ctx,"SELECT watermark_signature FROM security_soc_pb02_watch_cursors").Scan(&latest);err!=nil||latest!=sig2{
        t.Fatalf("failed fetch advanced watermark: %s %v",latest,err)
    }
    var enforcement,integrity string
    if err:=db.QueryRowContext(ctx,"SELECT enforcement_state, report_integrity FROM security_soc_pb02_authority_observations LIMIT 1").Scan(&enforcement,&integrity);err!=nil||enforcement!="not_connected"||integrity!="unsigned_rpc_observation"{
        t.Fatalf("observed authority upgraded into enforced verdict: %s %s %v",enforcement,integrity,err)
    }
    if calls.Load()<5{t.Fatalf("RPC path did not execute: calls=%d",calls.Load())}
}
