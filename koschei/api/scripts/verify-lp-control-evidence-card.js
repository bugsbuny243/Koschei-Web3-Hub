'use strict';

global.window={};
require('../public/js/lp-control-evidence-card.js');
const card=global.window.KoscheiLPControlCard;
if(!card||typeof card.render!=='function')throw new Error('LP control card API missing');

const pump=card.render({lp_control:{
  status:'burned',pool_type:'pumpswap_amm',control_model:'lp_token',position_model:'fungible_lp_token',
  pool_address:'PumpPool111',pool_program:'pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA',read_slot:900,canonical_pool:true,
  token_vault:'TokenVault111',quote_vault:'QuoteVault111',token_reserve:2000,quote_reserve:100,virtual_quote_reserve:5,
  reserve_liquidity_usd:4000,lp_mint:'LPMint111',lp_supply:1000,burned_share_pct:90,creator_lp_share_pct:0,
  movement_status:'observed',movement_window_signatures:20,movement_window_parsed:12,movement_window_failures:0,
  liquidity_movements:[{kind:'remove_liquidity',signature:'RemoveLiquiditySignature111',slot:901,block_time:'2026-07-17T18:00:00Z',actor_wallet:'Actor111',token_delta:-50,quote_delta:-10}]
}},{lang:'tr'});
for(const expected of ['LİKİDİTE KONTROL KANITI','pumpswap_amm','PumpPool','TokenVault','90%','REMOVE LIQUIDITY','RemoveLiquidity','slot 901']){
  if(!pump.includes(expected))throw new Error(`PumpSwap card missing: ${expected}`);
}
if(pump.includes('veri yok')||pump.includes('analiz yapılmadı'))throw new Error('empty narrative leaked into LP card');

const meteora=card.render({lp_control:{
  status:'permanently_locked',pool_type:'meteora_damm_v2',control_model:'position_nft',position_model:'meteora_damm_v2_position_nft',
  pool_address:'MeteoraPool111',pool_program:'cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG',read_slot:1000,
  token_vault:'VaultA',quote_vault:'VaultB',token_reserve:5000,quote_reserve:250,
  pool_liquidity_raw:'1000',permanent_locked_liquidity_raw:'250',permanent_locked_share_pct:25,
  movement_status:'complete_no_movement_observed',movement_window_signatures:10,movement_window_parsed:8,movement_window_failures:0,liquidity_movements:[]
}},{lang:'tr'});
for(const expected of ['meteora_damm_v2','POSITION NFT','25%','8 parsed','10 signatures']){
  if(!meteora.includes(expected))throw new Error(`Meteora card missing: ${expected}`);
}
if(meteora.includes('LP mint</label>'))throw new Error('position-NFT pool rendered an LP mint field');
if(!card.render({lp_control:{status:'not_applicable'}})==='')throw new Error('not-applicable pool should not render without an address');

console.log('LP control evidence card contract: ok');
