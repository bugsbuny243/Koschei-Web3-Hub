package services

import (
	"testing"
	"time"
)

func c005Holder(share float64, resolved bool) HolderIntelligence {
	rows := []HolderIntelligenceRow{}
	if resolved {
		rows = append(rows, HolderIntelligenceRow{OwnerWallet:"Owner111",OwnerResolved:true,RiskBearing:true,ExcludedFromHolderRisk:false,CirculatingPercentage:share})
	}
	return HolderIntelligence{Available:true,OwnerAggregationApplied:resolved,CirculatingSupply:1000,TopOwnerPercentage:share,Rows:rows}
}

func TestC005SeventyTwoPercentCapsAtF(t *testing.T) {
	behavior:=ApplyOwnerConcentrationRuleV110(UnifiedRadarBehaviorReport{Mint:"Mint72",Signals:[]UnifiedRadarSignal{},Evidence:[]ActorDefenseEvidenceRecord{}},c005Holder(72,true),time.Now())
	verdict:=EvaluateUnifiedRadarVerdictV110("Mint72",ActorDefenseRuleVerdict{Grade:"-",TriggeredRules:[]ActorDefenseRuleHit{},WatchFlags:[]ActorDefenseRuleHit{}},behavior)
	if verdict.Grade!="F"||verdict.RulesetVersion!=UnifiedRadarRulesetVersionV110||!verdict.Signed{t.Fatalf("verdict=%#v",verdict)}
}

func TestC005FiftyFivePercentCapsAtD(t *testing.T) {
	behavior:=ApplyOwnerConcentrationRuleV110(UnifiedRadarBehaviorReport{Mint:"Mint55",Signals:[]UnifiedRadarSignal{},Evidence:[]ActorDefenseEvidenceRecord{}},c005Holder(55,true),time.Now())
	verdict:=EvaluateUnifiedRadarVerdictV110("Mint55",ActorDefenseRuleVerdict{Grade:"-",TriggeredRules:[]ActorDefenseRuleHit{},WatchFlags:[]ActorDefenseRuleHit{}},behavior)
	if verdict.Grade!="D"||!verdict.Signed{t.Fatalf("verdict=%#v",verdict)}
}

func TestC005RawOnlySeventyPercentDoesNotTrigger(t *testing.T) {
	behavior:=ApplyOwnerConcentrationRuleV110(UnifiedRadarBehaviorReport{Mint:"Raw70",Signals:[]UnifiedRadarSignal{},Evidence:[]ActorDefenseEvidenceRecord{}},c005Holder(70,false),time.Now())
	last:=behavior.Signals[len(behavior.Signals)-1]
	if last.Triggered||last.EvidenceStatus!="unverified"{t.Fatalf("signal=%#v",last)}
	verdict:=EvaluateUnifiedRadarVerdictV110("Raw70",ActorDefenseRuleVerdict{Grade:"-",TriggeredRules:[]ActorDefenseRuleHit{},WatchFlags:[]ActorDefenseRuleHit{}},behavior)
	if verdict.Grade!="-"||verdict.Signed{t.Fatalf("verdict=%#v",verdict)}
}
