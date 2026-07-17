'use strict';

const card=require('../public/js/unified-live-evidence-card.js');

const fixture={
  full_scan_live_evidence:{
    status:'complete',
    wallets_requested:2,
    wallets_completed:2,
    signatures_seen:34,
    transactions_parsed:17,
    rpc_failures:0,
    launch_signer:{
      available:true,
      status:'observed_launch_transaction_signer',
      wallet:'LaunchSigner111111111111111111111111111111',
      signature:'LaunchSignature111111111111111111111111111111111111',
      slot:100,
      instruction_types:['initializeMint']
    },
    wallet_coverage:[
      {wallet:'Creator111',role:'creator_source_observed',status:'complete',signatures_seen:20,transactions_parsed:10,relevant_transactions:1},
      {wallet:'Owner111',role:'risk_bearing_holder',status:'complete',signatures_seen:14,transactions_parsed:7,relevant_transactions:1}
    ],
    transactions:[
      {
        signature:'CreatorSellSignature111111111111111111111111111111',slot:201,block_time:'2026-07-17T09:00:00Z',
        wallet:'Creator111',role:'creator_source_observed',direction:'sell',token_delta:-42.5,swap_related:true,
        counterparties:['Pool111'],instruction_types:['swap'],token_mints:['Mint111'],evidence_key:'creator-sell-evidence',source:'solana_jsonparsed_manual_full_scan'
      },
      {
        signature:'OwnerTransferSignature111111111111111111111111111',slot:202,block_time:'2026-07-17T09:01:00Z',
        wallet:'Owner111',role:'risk_bearing_holder',direction:'transfer_out',token_delta:-12,swap_related:false,
        counterparties:['Recipient111'],instruction_types:['transferChecked'],token_mints:['Mint111'],evidence_key:'owner-transfer-evidence',source:'solana_jsonparsed_manual_full_scan'
      }
    ]
  }
};

const html=card.render(fixture,{lang:'tr'});
for(const expected of ['2 doğrulanabilir işlem satırı','Creator kaynağı','SATIŞ / SWAP ÇIKIŞI','-42,5','CreatorSell','slot 201','Launch işlem imzacısı gözlendi']){
  if(!html.includes(expected))throw new Error(`live evidence HTML missing: ${expected}`);
}
if(html.includes('veri yok')||html.includes('analiz yapılmadı'))throw new Error('empty narrative leaked into live evidence card');

const empty=card.render({full_scan_live_evidence:{
  status:'complete',wallets_requested:2,wallets_completed:2,signatures_seen:20,transactions_parsed:12,rpc_failures:0,
  launch_signer:{available:false},wallet_coverage:[],transactions:[]
}});
if(!empty.includes('12 işlem ayrıştırıldı'))throw new Error('empty bounded window did not show parsed transaction count');
if(!empty.includes('eski hareketlerin yokluğu anlamına gelmez'))throw new Error('bounded-window limitation missing');
if(empty.includes('<table'))throw new Error('empty live evidence rendered an empty table');

const skipped=card.render({full_scan_live_evidence:{status:'not_requested'}});
if(skipped!=='')throw new Error('not-requested live evidence should not render');

console.log('live evidence card contract: ok');
