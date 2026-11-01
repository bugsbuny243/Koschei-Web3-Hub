package services

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "log"
    "os"
    "strings"
    "time"
)

const (
    socPB02WatchLimit = 4
    socPB02PageSize = 30
    socPB02MaxPages = 4
    socPB02TokenProgram = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
    socPB02Token2022Program = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
)

type socPB02WatchEvent struct {
    Watch string
    Signature string
    Slot int64
    Source string
    Index int
    InnerSequence int
    ProgramID string
    Kind string
    AuthorityType string
    NewAuthority string
    Severity string
    Recommendation string
}

type socPB02AuthorityWatch struct {
    DB *sql.DB
    RPCURL string
    Addresses []string
    PollEvery time.Duration
}

func parseSOCWatchAddresses(raw string) ([]string,error) {
    raw = strings.TrimSpace(raw)
    if raw == "" { return nil,errors.New("watched Solana addresses are not configured") }
    parts := strings.Split(raw,",")
    if len(parts) > socPB02WatchLimit { return nil,errors.New("more than four Solana watch accounts are unsupported") }
    out := make([]string,0,len(parts))
    seen := map[string]bool{}
    for _, part := range parts {
        addr := strings.TrimSpace(part)
        if !isLikelyRadarSolanaAddress(addr) || seen[addr] { return nil,fmt.Errorf("invalid or duplicate watch address") }
        seen[addr] = true
        out = append(out,addr)
    }
    return out,nil
}

// This opt-in worker never enables the unbounded WSS PostgreSQL journal.
// Signer enforcement and signed SOC case creation are deliberately not claimed.
func StartSOCpb02AuthorityWatchIfEnabled(ctx context.Context, db *sql.DB) func() {
    if !envBool("KOSCHEI_SOC_PB02_WATCH_ENABLED") { return func(){} }
    if db == nil || !AutomaticBackgroundScanningEnabled() {
        log.Printf("SOC-PB-02 watch withheld: database or background scanning unavailable")
        return func(){}
    }
    watches,err := parseSOCWatchAddresses(os.Getenv("KOSCHEI_SOC_PB02_WATCH_ADDRESSES"))
    if err != nil { log.Printf("SOC-PB-02 watch withheld: %v",err);return func(){} }
    rpcURL := resolveSecurityRadarRPCURL()
    if rpcURL == "" {log.Printf("SOC-PB-02 watch withheld: Solana RPC unavailable");return func(){}}
    worker := &socPB02AuthorityWatch{
        DB: db,RPCURL:rpcURL,Addresses:watches,
        PollEvery: time.Duration(boundedSecurityRadarEnvInt("KOSCHEI_SOC_PB02_WATCH_INTERVAL_SECONDS",60,30,300))*time.Second,
    }
    watchCtx,cancel := context.WithCancel(ctx)
    go worker.run(watchCtx)
    return cancel
}

func (w *socPB02AuthorityWatch) run(ctx context.Context) {
    log.Printf("SOC-PB-02 bounded watch started addresses=%d interval=%s; raw WSS journal remains independently gated",len(w.Addresses),w.PollEvery)
    w.runOnce(ctx)
    ticker := time.NewTicker(w.PollEvery)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():return
        case <-ticker.C:w.runOnce(ctx)
        }
    }
}

func (w *socPB02AuthorityWatch) runOnce(ctx context.Context) {
    if w==nil||w.DB==nil||w.RPCURL=="" {return}
    for _,watch := range w.Addresses {
        if ctx.Err()!=nil {return}
        if err:=w.pollAddress(ctx,watch);err!=nil&&ctx.Err()==nil {
            log.Printf("SOC-PB-02 watch incomplete address=%s; cursor withheld: %s",watch,compactRadarError("pb02 watch",err))
        }
    }
}

func (w *socPB02AuthorityWatch) getSignatures(ctx context.Context,addr,before string) ([]SolanaSignatureInfo,error) {
    options:=map[string]any{"limit":socPB02PageSize,"commitment":"confirmed"}
    if before!="" {options["before"]=before}
    return solanaRPCDo[[]SolanaSignatureInfo](ctx,w.RPCURL,"getSignaturesForAddress",[]any{addr,options})
}

func (w *socPB02AuthorityWatch) cursor(ctx context.Context,addr string) (string,error) {
    var sig string
    err:=w.DB.QueryRowContext(ctx,`SELECT watermark_signature FROM security_soc_pb02_watch_cursors WHERE network='solana-mainnet' AND watch_address=$1`,addr).Scan(&sig)
    if errors.Is(err,sql.ErrNoRows) {return "",nil}
    return sig,err
}

func (w *socPB02AuthorityWatch) pollAddress(ctx context.Context,addr string) error {
    watermark,err:=w.cursor(ctx,addr)
    if err!=nil{return err}
    first,err:=w.getSignatures(ctx,addr,"")
    if err!=nil{return err}
    if len(first)==0 {return nil}
    for _,info:=range first {if !securityRadarValidSignature(info.Signature)||info.Slot<=0 {return errors.New("unverified signature page")}}
    if watermark=="" {
        // First start is explicitly prospective. Historical coverage is unknown.
        return w.commitSignature(ctx,addr,"",first[0],nil,true)
    }
    // Never skip entries when a provider page is full, incomplete, or when
    // a prior watermark is outside the bounded 120-signature search window.
    gathered:=make([]SolanaSignatureInfo,0,socPB02PageSize)
    before:=""
    reached:=false
    for page:=0;page<socPB02MaxPages;page++ {
        var batch []SolanaSignatureInfo
        if page==0 {batch=first} else {
            batch,err=w.getSignatures(ctx,addr,before)
            if err!=nil{return err}
        }
        if len(batch)==0 {return errors.New("watermark disappeared before complete replay")}
        for _,info:=range batch {
            if !securityRadarValidSignature(info.Signature)||info.Slot<=0 {return errors.New("invalid signature in replay page")}
            if info.Signature==watermark {reached=true;break}
            gathered=append(gathered,info)
        }
        if reached {break}
        if len(batch)<socPB02PageSize {return errors.New("watermark absent from complete provider history")}
        before=batch[len(batch)-1].Signature
    }
    if !reached {return errors.New("bounded signature window exhausted; gap unresolved")}
    // Process oldest first. A persisted alert and cursor update share one DB
    // transaction. If any getTransaction/parse/write fails, stop immediately.
    prev:=watermark
    for i:=len(gathered)-1;i>=0;i-- {
        info:=gathered[i]
        events,decodeErr:=w.inspectTransaction(ctx,addr,info)
        if decodeErr!=nil{return decodeErr}
        if err:=w.commitSignature(ctx,addr,prev,info,events,false);err!=nil{return err}
        prev=info.Signature
    }
    return nil
}

func (w *socPB02AuthorityWatch) inspectTransaction(ctx context.Context,watch string,info SolanaSignatureInfo) ([]socPB02WatchEvent,error) {
    tx,err:=SolanaGetTransactionJSONParsed(ctx,w.RPCURL,info.Signature)
    if err!=nil{return nil,err}
    return inspectSOCpb02ParsedTransaction(watch,info,tx)
}

// Exact confirmed signature/slot/account + nested CPI provenance are required.
// Unknown token instructions are not silently treated as harmless.
func inspectSOCpb02ParsedTransaction(watch string,info SolanaSignatureInfo,tx SolanaTransactionResult) ([]socPB02WatchEvent,error) {
    if !isLikelyRadarSolanaAddress(watch)||!securityRadarValidSignature(info.Signature)||info.Slot<=0 {
        return nil,errors.New("invalid PB02 subject provenance")
    }
    if int64(radarInt64(tx["slot"]))!=info.Slot {return nil,errors.New("getTransaction slot mismatch")}
    meta:=asRadarMap(tx["meta"])
    transaction:=asRadarMap(tx["transaction"])
    message:=asRadarMap(transaction["message"])
    signatures,ok:=transaction["signatures"].([]any)
    if !ok||len(signatures)==0||anyString(signatures[0])!=info.Signature {
        return nil,errors.New("getTransaction signature mismatch")
    }
    errValue,hasError:=meta["err"]
    if !hasError {return nil,errors.New("transaction execution status unavailable")}
    keys,ok:=message["accountKeys"].([]any)
    if !ok||len(keys)==0 {return nil,errors.New("transaction account keys unavailable")}
    watchIncluded:=false
    for _,entry:=range keys {
        key:=anyString(entry)
        if m,ok:=entry.(map[string]any);ok {key=anyString(m["pubkey"])}
        if key==watch {watchIncluded=true}
    }
    if !watchIncluded {return nil,errors.New("watched account absent from transaction keys")}
    if (info.Err!=nil)!=(errValue!=nil) {return nil,errors.New("signature and transaction status mismatch")}
    if errValue!=nil {return []socPB02WatchEvent{},nil}
    outer,ok:=message["instructions"].([]any)
    if !ok {return nil,errors.New("outer instruction evidence unavailable")}
    innerRaw,exists:=meta["innerInstructions"]
    if !exists||innerRaw==nil {return nil,errors.New("inner CPI evidence unavailable")}
    inner,ok:=innerRaw.([]any)
    if !ok {return nil,errors.New("malformed inner CPI groups")}
    events:=make([]socPB02WatchEvent,0,2)
    examined:=0
    inspect:=func(raw any,source string,index,seq int) error {
        examined++
        if examined>512 {return errors.New("transaction instruction cap exceeded")}
        instruction:=asRadarMap(raw)
        programID:=anyString(instruction["programId"])
        if programID!=socPB02TokenProgram&&programID!=socPB02Token2022Program {return nil}
        parsed:=asRadarMap(instruction["parsed"])
        kind:=anyString(parsed["type"])
        if kind=="" {return errors.New("token instruction did not decode")}
        if kind!="setAuthority"&&kind!="initializePermanentDelegate" {return nil}
        infoMap:=asRadarMap(parsed["info"])
        target:=anyString(infoMap["account"])
        if target=="" {target=anyString(infoMap["mint"])}
        if !isLikelyRadarSolanaAddress(target) {return errors.New("token authority target unresolved")}
        if target!=watch {return nil}
        authorityType:=anyString(infoMap["authorityType"])
        newAuthority:=anyString(infoMap["newAuthority"])
        revoked:=false
        if kind=="initializePermanentDelegate" {
            if programID!=socPB02Token2022Program{return errors.New("permanent delegate on non-Token2022 program")}
            authorityType="permanentDelegate"
            newAuthority=anyString(infoMap["delegate"])
            if newAuthority=="" {return errors.New("permanent delegate identity missing")}
        } else {
            if authorityType=="" {return errors.New("SetAuthority type missing")}
            raw,exists:=infoMap["newAuthority"]
            if !exists {return errors.New("new authority presence not verified")}
            revoked=raw==nil
            if !revoked&&newAuthority=="" {return errors.New("empty new authority")}
        }
        if newAuthority!=""&&!isLikelyRadarSolanaAddress(newAuthority){return errors.New("new authority malformed")}
        severity:="high"
        recommendation:="owner_review_authorization"
        if revoked {
            severity="informational"
            recommendation="record_authority_revocation"
        } else {
            switch strings.ToLower(authorityType) {
            case "minttokens","freezeaccount","permanentdelegate","pausetransactions":severity="critical"
            }
        }
        events=append(events,socPB02WatchEvent{Watch:watch,Signature:info.Signature,Slot:info.Slot,Source:source,
            Index:index,InnerSequence:seq,ProgramID:programID,Kind:kind,AuthorityType:authorityType,NewAuthority:newAuthority,Severity:severity,Recommendation:recommendation})
        return nil
    }
    for i,raw:=range outer {if err:=inspect(raw,"outer",i,0);err!=nil{return nil,err}}
    for _,groupRaw:=range inner {
        group:=asRadarMap(groupRaw)
        idx,ok:=group["index"].(float64)
        if !ok||idx<0||idx>=float64(len(outer))||idx!=float64(int(idx)){return nil,errors.New("inner parent instruction invalid")}
        nested,ok:=group["instructions"].([]any)
        if !ok{return nil,errors.New("inner CPI instructions unavailable")}
        for j,raw:=range nested {if err:=inspect(raw,"inner",int(idx),j);err!=nil{return nil,err}}
    }
    return events,nil
}

func (w *socPB02AuthorityWatch) commitSignature(ctx context.Context,watch,previous string,info SolanaSignatureInfo,events []socPB02WatchEvent,bootstrap bool) error {
    dbtx,err:=w.DB.BeginTx(ctx,nil)
    if err!=nil{return err}
    defer dbtx.Rollback()
    for _,event:=range events {
        _,err=dbtx.ExecContext(ctx,`INSERT INTO security_soc_pb02_authority_observations
            (network,watch_address,signature,slot,instruction_source,instruction_index,inner_sequence,program_id,event_kind,authority_type,new_authority,severity,recommended_action)
            VALUES ('solana-mainnet',$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT DO NOTHING`,
            watch,event.Signature,event.Slot,event.Source,event.Index,event.InnerSequence,event.ProgramID,event.Kind,event.AuthorityType,event.NewAuthority,event.Severity,event.Recommendation)
        if err!=nil{return err}
    }
    if bootstrap {
        _,err=dbtx.ExecContext(ctx,`INSERT INTO security_soc_pb02_watch_cursors
            (network,watch_address,watermark_signature,watermark_slot,bootstrap_only)
            VALUES ('solana-mainnet',$1,$2,$3,true) ON CONFLICT DO NOTHING`,watch,info.Signature,info.Slot)
        if err!=nil{return err}
    } else {
        var updated string
        err=dbtx.QueryRowContext(ctx,`UPDATE security_soc_pb02_watch_cursors
            SET watermark_signature=$3,watermark_slot=$4,last_checked_at=now(),bootstrap_only=false
            WHERE network='solana-mainnet' AND watch_address=$1 AND watermark_signature=$2
            RETURNING watermark_signature`,watch,previous,info.Signature,info.Slot).Scan(&updated)
        if err!=nil{return fmt.Errorf("concurrent cursor update or missing cursor: %w",err)}
    }
    return dbtx.Commit()
}
