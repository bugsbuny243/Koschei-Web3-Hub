package services

import (
 "testing"
 "time"

 "koschei/api/internal/radaradapter"
)

func TestAdaptRadarEvidenceObservedOnly(t *testing.T) {
 subject:=IntelligenceSubject{ID:"subject-1",CanonicalRef:"fixture:account",ChainFamily:IntelligenceChainFamilyEVM,Chain:"ethereum",Network:"ethereum-mainnet"}
 identity:=radaradapter.SubjectIdentity{Network:"ethereum-mainnet",IdentityKind:"chain_id",IdentityValue:"1",NativeID:"account"}
 now:=time.Date(2026,10,11,0,0,0,0,time.UTC)
 rows:=[]radaradapter.EvidenceRow{{ID:"evidence-1",SubjectID:"subject-1",Status:"verified",Source:"fixture-provider",Height:"123",RawHash:"sha256:fixture",ObservedAt:now,Attributes:map[string]any{"native_chain_identity":"tampered"}}}
 evidence,err:=AdaptRadarEvidence(subject,identity,rows)
 if err!=nil {t.Fatal(err)}
 if len(evidence)!=1 || evidence[0].Status!=IntelligenceEvidenceObserved {t.Fatal("adapter must never self-verify")}
 if evidence[0].Attributes["native_chain_identity"]!="1" {t.Fatal("adapter overwrote trusted identity")}
 if evidence[0].BlockOrSlot!=123 {t.Fatal("height mapping failed")}
}

func TestAdaptRadarEvidenceRejectsMismatch(t *testing.T) {
 subject:=IntelligenceSubject{ID:"subject-1",CanonicalRef:"fixture:account",Network:"ethereum-mainnet"}
 identity:=radaradapter.SubjectIdentity{Network:"ethereum-mainnet",IdentityKind:"chain_id",IdentityValue:"1",NativeID:"account"}
 rows:=[]radaradapter.EvidenceRow{{ID:"evidence-1",SubjectID:"wrong-subject",Source:"fixture-provider",Height:"123",RawHash:"sha256:fixture",ObservedAt:time.Now()}}
 if _,err:=AdaptRadarEvidence(subject,identity,rows);err==nil {t.Fatal("mismatched subject accepted")}
}
