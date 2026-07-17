'use strict';

const card=require('../public/js/verdict-card-evidence-refs.js');

const rowIDs=[
  'launch','mint','freeze','wash','address','liquidity','funding','concentration','sniper','first-buyer',
  'track','creator-sell','dominant-exit','liq-move','program','metadata','claim','mev','distribution','signed'
];

function references(){
  return Object.fromEntries(rowIDs.map(id=>[id,{
    wallets:id==='concentration'?['Owner111']:[],
    accounts:['Mint111'],
    signatures:id==='signed'?['VerdictSignature111']:[],
    slots:id==='launch'?[100]:[],
    evidence_keys:[`row:${id}`]
  }]));
}

function fixture(){
  return{
    target:'Mint111',
    generated_at:'2026-07-17T07:00:00Z',
    final_verdict:{grade:'D',verdict:'hard_trigger',signed:true,signature:'VerdictSignature111',ruleset_version:'koschei-unified-radar-rules-v1.1.0',generated_at:'2026-07-17T07:00:00Z'},
    holder_intelligence:{available:true,owner_aggregation_applied:true,circulating_supply:1000000,top_owner_percentage:55,rows:[{owner_wallet:'Owner111',owner_resolved:true,risk_bearing:true,excluded_from_holder_risk:false,token_accounts:['OwnerATA111']}]},
    holder_distribution:{top_1_percentage:55},
    holder_concentration_context:{available:true,status:'observed_corpus_percentile',top_share_pct:55,top_percentile:7.25,sample_count:50000,bucket_width:1,method:'distinct_mint_latest_owner_resolved_top_share_histogram'},
    market:{available:true,liquidity_usd:100000,best_pair_address:'Pool111'},
    lp_control:{available:true,status:'burned',pool_address:'Pool111',lp_mint:'LP111',token_vault:'Vault111',quote_vault:'Quote111',read_slot:200,burned_share_pct:99,evidence_keys:['pool:Pool111@200']},
    launch_forensics:{available:true,launch_time:'2026-07-17T06:00:00Z',launch_slot:100,owners_requested:1,owners_with_trade_history:1,ledger_trade_count:1,sniper_count:0,creator_linked_count:0,profiles:[]},
    trade_ledger_aggregates:{available:true,trade_count:1,round_trip_wallet_count:0},
    actor_investigation:{dossier:{tokens:[],related_actors:[],evidence:[]}},
    behavior_signals:{signals:[{rule_id:'URD-C005',evidence_status:'verified',triggered:true,summary:'Owner concentration observed.',evidence_keys:['owner:Owner111']}]},
    modules:[
      {module_id:'token_authority_scanner',signals:{execution_status:'completed',mint_authority_present:false,freeze_authority_present:false},verified:true},
      {module_id:'holder_concentration',signals:{execution_status:'completed',top_owner_percentage:55},verified:true},
      {module_id:'liquidity_movement',signals:{execution_status:'completed',liquidity_usd:100000},verified:true},
      {module_id:'launch_distribution',signals:{execution_status:'completed'},verified:true}
    ],
    evidence_references:references()
  };
}

const vm=card.mapVerdictCard(fixture(),{lang:'tr'});
if(!vm||!Array.isArray(vm.checklist)||vm.checklist.length!==20)throw new Error(`expected 20 rows, got ${vm?.checklist?.length}`);
for(const row of vm.checklist){
  if(!card.refsPresent(row.refs))throw new Error(`row ${row.id} has no evidence reference`);
}
const concentration=vm.checklist.find(row=>row.id==='concentration');
if(!concentration.refs.wallets.includes('Owner111'))throw new Error('owner wallet reference missing');
if(!String(concentration.value).includes('50.000'))throw new Error(`corpus sample count missing: ${concentration.value}`);
if(!String(concentration.value).includes('7,25'))throw new Error(`corpus percentile missing: ${concentration.value}`);
if(concentration.corpus_context?.method!=='distinct_mint_latest_owner_resolved_top_share_histogram')throw new Error('corpus method missing');
const signed=vm.checklist.find(row=>row.id==='signed');
if(!signed.refs.signatures.includes('VerdictSignature111'))throw new Error('verdict signature reference missing');

const missing=fixture();
missing.evidence_references.concentration={wallets:[],accounts:[],signatures:[],slots:[],evidence_keys:[]};
const degraded=card.mapVerdictCard(missing,{lang:'tr'}).checklist.find(row=>row.id==='concentration');
if(degraded.state!=='arm_pending')throw new Error(`missing evidence reference did not degrade row: ${degraded.state}`);
if(!String(degraded.value).includes('Kanıt referansı'))throw new Error('degraded row did not explain reference gap');

const smallCorpus=fixture();
smallCorpus.holder_concentration_context={available:false,status:'corpus_sample_too_small',sample_count:12,top_share_pct:55};
const withheld=card.mapVerdictCard(smallCorpus,{lang:'tr'}).checklist.find(row=>row.id==='concentration');
if(String(withheld.value).includes('farklı mint corpus'))throw new Error('small corpus rendered a percentile');

console.log('verdict-card evidence reference and corpus contract: ok');
