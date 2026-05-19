import "server-only";
import { web3Db } from "@/lib/web3-db";

export const gameFactoryPositioning = "Koschei Web Game Factory turns a plain-language game prompt into a playable HTML5 demo and a Web3-ready package.";
export const gameFactorySafetyCopy = "Koschei Web3 Bridge MVP does not hold funds, manage private keys, connect wallets, deploy contracts, mint NFTs, sign transactions, or custody user assets. It only prepares game manifests, item schemas, NFT-compatible metadata, reward configs, and adapter configs.";

export type GFProject = { id:string; title:string|null; prompt:string; genre:string|null; visual_style:string|null; target_chain:string; status:string; metadata:Record<string,unknown>; created_at:string; updated_at:string };
export type TemplateKey = "quest_arena_rpg"|"arena_survival"|"tower_defense"|"boss_fight"|"platformer"|"card_battle"|"resource_management"|"runner";

type GameBrief = {
  title:string; genre:string; style:string; prompt:string; template:TemplateKey;
  player:string; collectible:string; obstacle:string; goal:string; score_rule:string; game_over:string;
  labels:string[];
  web3:{ item_schema:string[]; nft_traits:string[]; reward_events:{key:string;reward:number}[]; notes?:string[] };
};

type SceneConfig = { template: TemplateKey; title: string; stats: Record<string, number>; labels: string[] };

export function slugify(v:string){return v.toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/(^-|-$)/g,"") || "game-project";}
const hasAny = (text:string, kws:string[]) => kws.some((k)=>text.includes(k));

export function detectGameTemplate(prompt:string, genre?:string|null):TemplateKey {
  const text = `${prompt} ${genre ?? ""}`.toLowerCase();
  if (hasAny(text,["quest","badge","guardian","arena","chest","rpg","relic","guild"])) return "quest_arena_rpg";
  if (hasAny(text,["survival","swarm","wave","enemies spawn","xp"])) return "arena_survival";
  if (hasAny(text,["tower defense","towers","base hp","waves","credits"])) return "tower_defense";
  if (hasAny(text,["boss","titan","boss hp","phases","meteors","laser"])) return "boss_fight";
  if (hasAny(text,["platformer","jump","platforms","spikes","portal"])) return "platformer";
  if (hasAny(text,["card","deck","turn","attack","shield"])) return "card_battle";
  if (hasAny(text,["colony","resource","mining","solar","shield","tycoon"])) return "resource_management";
  return "runner";
}

export function buildGameBrief(input:{title?:string|null;prompt:string;genre?:string|null;style?:string|null}):GameBrief{
  const title=input.title?.trim()||"Prompt Runner";const genre=input.genre?.trim()||"arcade";const style=input.style?.trim()||"minimal neon";
  const template = detectGameTemplate(input.prompt, input.genre);
  const common = {title,genre,style,prompt:input.prompt};
  if (template === "quest_arena_rpg") return {...common,template,player:"hero",collectible:"ancient badge",obstacle:"guardian enemy",goal:"collect 3 badges and unlock final chest",score_rule:"+15 per badge, +30 per guardian",game_over:"HP reaches 0",labels:["hero","ancient badge","guardian","final chest"],web3:{item_schema:["ancient_badge","guardian_reward","final_treasure_chest"],nft_traits:["badge","quest"],reward_events:[{key:"collect_3_badges",reward:60},{key:"defeat_guardians",reward:40},{key:"unlock_final_chest",reward:100}],notes:["quest_progress_enabled"]}};
  if (template === "arena_survival") return {...common,template,player:"arena fighter",collectible:"xp orb",obstacle:"swarm enemy",goal:"survive escalating waves",score_rule:"+5 per orb, +20 per enemy",game_over:"HP reaches 0",labels:["arena fighter","xp orb","wave","enemy swarm"],web3:{item_schema:["xp_orb","wave_reward"],nft_traits:["survival","wave"],reward_events:[{key:"clear_wave",reward:25},{key:"collect_xp",reward:5}]}};
  if (template === "tower_defense") return {...common,template,player:"tower commander",collectible:"credits",obstacle:"path enemy",goal:"defend base across waves",score_rule:"+10 per defeated enemy",game_over:"Base HP reaches 0",labels:["tower slot","tower","enemy path","base hp"],web3:{item_schema:["tower_core","defense_credits"],nft_traits:["defense","tower"],reward_events:[{key:"defeat_enemy",reward:10},{key:"survive_wave",reward:30}]}};
  if (template === "boss_fight") return {...common,template,player:"player ship",collectible:"repair orb",obstacle:"titan boss",goal:"defeat boss before HP depletion",score_rule:"+20 per boss hit",game_over:"Player HP reaches 0",labels:["player ship","titan boss","boss hp","repair orb"],web3:{item_schema:["repair_orb","boss_core"],nft_traits:["boss","phase"],reward_events:[{key:"survive_phase",reward:30},{key:"defeat_boss",reward:150}]}};
  if (template === "platformer") return {...common,template,player:"platform explorer",collectible:"crystal",obstacle:"spike trap",goal:"collect crystals then reach portal",score_rule:"+10 per crystal",game_over:"HP reaches 0",labels:["platform","crystal","spike","portal"],web3:{item_schema:["crystal","portal_pass"],nft_traits:["platformer","jump"],reward_events:[{key:"collect_crystal",reward:10},{key:"reach_portal",reward:70}]}};
  if (template === "card_battle") return {...common,template,player:"duelist",collectible:"energy",obstacle:"enemy champion",goal:"reduce enemy HP to zero",score_rule:"+15 per winning turn",game_over:"Player HP reaches 0",labels:["attack card","shield card","energy blast","turn"],web3:{item_schema:["battle_card","energy_charge"],nft_traits:["card","turn"],reward_events:[{key:"win_turn",reward:15},{key:"win_match",reward:100}]}};
  if (template === "resource_management") return {...common,template,player:"colony overseer",collectible:"minerals",obstacle:"hazard event",goal:"grow colony while shielding base",score_rule:"+8 per production cycle",game_over:"Shield HP reaches 0",labels:["energy","minerals","shield hp","colony level"],web3:{item_schema:["mineral_cache","solar_grid"],nft_traits:["resource","colony"],reward_events:[{key:"upgrade_colony",reward:35},{key:"mine_resources",reward:8}]}};
  return {...common,template,player:"runner",collectible:"star shard",obstacle:"void spike",goal:"collect and survive",score_rule:"+10 per collectible",game_over:"3 collisions",labels:["runner","star shard","void spike"],web3:{item_schema:["star_shard","prompt_pioneer_badge"],nft_traits:["runner","arcade"],reward_events:[{key:"collect",reward:10},{key:"game_complete",reward:50}]}};
}

export function buildGameSceneConfig(brief: GameBrief): SceneConfig {
  const statsByTemplate: Record<TemplateKey, Record<string, number>> = {
    quest_arena_rpg: { hp: 5, score: 0, badges: 0, chest: 0 }, arena_survival: { hp: 5, score: 0, wave: 1 },
    tower_defense: { baseHp: 20, credits: 100, score: 0, wave: 1 }, boss_fight: { hp: 5, score: 0, bossHp: 240 },
    platformer: { hp: 5, score: 0 }, card_battle: { playerHp: 30, enemyHp: 30, turn: 1 }, resource_management: { energy: 20, minerals: 10, shieldHp: 30, colonyLevel: 1 },
    runner: { hp: 3, score: 0 }
  };
  return { template: brief.template, title: brief.title, labels: brief.labels, stats: statsByTemplate[brief.template] };
}

export function buildGameAssets(brief: GameBrief){const b=brief;const out:Array<{asset_type:string;name:string;description:string;rarity:string;metadata:Record<string,unknown>}>=[{asset_type:"player",name:b.player,description:"Controllable character",rarity:"common",metadata:{template:b.template}}];
  if (b.template==="quest_arena_rpg") out.push({asset_type:"collectible",name:"ancient badge",description:"Quest badge collectible",rarity:"uncommon",metadata:{quest:true}},{asset_type:"enemy",name:"guardian enemy",description:"Arena guardian",rarity:"uncommon",metadata:{}},{asset_type:"reward",name:"final chest",description:"Final quest chest",rarity:"rare",metadata:{unlock_after_badges:3}});
  else if (b.template==="arena_survival") out.push({asset_type:"collectible",name:"xp orb",description:"Experience orb",rarity:"common",metadata:{}},{asset_type:"enemy",name:"swarm enemy",description:"Chasing enemy",rarity:"common",metadata:{}},{asset_type:"reward",name:"wave cache",description:"Wave clear reward",rarity:"uncommon",metadata:{}});
  else if (b.template==="tower_defense") out.push({asset_type:"resource",name:"defense credits",description:"Build currency",rarity:"common",metadata:{}},{asset_type:"tower",name:"defense tower",description:"Path defense tower",rarity:"uncommon",metadata:{}},{asset_type:"enemy",name:"path enemy",description:"Marching enemy",rarity:"common",metadata:{}});
  else if (b.template==="boss_fight") out.push({asset_type:"boss",name:"titan boss",description:"Phase-based boss",rarity:"epic",metadata:{}},{asset_type:"collectible",name:"repair orb",description:"HP recovery orb",rarity:"common",metadata:{}},{asset_type:"reward",name:"boss core",description:"Boss defeat reward",rarity:"rare",metadata:{}});
  else if (b.template==="platformer") out.push({asset_type:"collectible",name:"crystal",description:"Platform collectible",rarity:"common",metadata:{}},{asset_type:"obstacle",name:"spike trap",description:"Damage hazard",rarity:"common",metadata:{}},{asset_type:"reward",name:"portal key",description:"Portal completion unlock",rarity:"uncommon",metadata:{}});
  else if (b.template==="card_battle") out.push({asset_type:"card",name:"attack card",description:"Damage card",rarity:"common",metadata:{}},{asset_type:"card",name:"shield card",description:"Defense card",rarity:"common",metadata:{}},{asset_type:"card",name:"energy blast",description:"Burst card",rarity:"uncommon",metadata:{}});
  else if (b.template==="resource_management") out.push({asset_type:"resource",name:"minerals",description:"Primary colony resource",rarity:"common",metadata:{}},{asset_type:"resource",name:"energy",description:"Power budget",rarity:"common",metadata:{}},{asset_type:"reward",name:"colony level crest",description:"Colony advancement reward",rarity:"uncommon",metadata:{}});
  else out.push({asset_type:"collectible",name:"star shard",description:"Score item",rarity:"common",metadata:{}},{asset_type:"obstacle",name:"void spike",description:"Avoid this",rarity:"uncommon",metadata:{}},{asset_type:"reward",name:"Prompt Pioneer Badge",description:"Generated MVP reward badge",rarity:"rare",metadata:{badge:true}});
  return out;
}

export function renderPreviewHtml(sceneConfig: SceneConfig){
  const serialized = JSON.stringify(sceneConfig);
  return `<!doctype html><html><body style="margin:0;background:#070b16;color:#fff;font-family:system-ui"><canvas id="c" width="760" height="420"></canvas><script>const scene=${serialized};const c=document.getElementById('c');const x=c.getContext('2d');const K={};onkeydown=e=>K[e.key]=1;onkeyup=e=>K[e.key]=0;let tick=0;function t(v,y){x.fillStyle='#fff';x.fillText(String(v),12,y);}function drawQuest(){let p={x:60,y:220},badges=[{x:180,y:90,t:0},{x:330,y:300,t:0},{x:520,y:180,t:0}],guards=[{x:420,y:90},{x:620,y:280}],chest={x:680,y:200,open:0},hp=5,score=0;function loop(){tick++;x.fillStyle='#0d1326';x.fillRect(0,0,760,420);x.font='14px sans-serif';t(scene.title,20);t('Badges '+badges.filter(b=>b.t).length+'/3 HP '+hp+' Score '+score,40);if(K.ArrowUp)p.y-=3;if(K.ArrowDown)p.y+=3;if(K.ArrowLeft)p.x-=3;if(K.ArrowRight)p.x+=3;p.x=Math.max(0,Math.min(740,p.x));p.y=Math.max(0,Math.min(400,p.y));for(const g of guards){g.x+=Math.sin((tick+g.y)/30)*1.2;if(Math.abs(p.x-g.x)<16&&Math.abs(p.y-g.y)<16){hp--;p.x=60;p.y=220;score=Math.max(0,score-5);}}for(const b of badges){if(!b.t&&Math.abs(p.x-b.x)<14&&Math.abs(p.y-b.y)<14){b.t=1;score+=15;}}if(badges.every(b=>b.t))chest.open=1;if(chest.open&&Math.abs(p.x-chest.x)<20&&Math.abs(p.y-chest.y)<20)t('VICTORY: final chest unlocked',64);x.fillStyle='#22d3ee';x.fillRect(p.x,p.y,20,20);x.fillStyle='#facc15';for(const b of badges){if(!b.t){x.beginPath();x.arc(b.x,b.y,8,0,6.28);x.fill();}}x.fillStyle='#ef4444';for(const g of guards)x.fillRect(g.x,g.y,18,18);x.fillStyle=chest.open?'#86efac':'#6b7280';x.fillRect(chest.x,chest.y,26,20);if(hp<=0)t('Game Over',82);requestAnimationFrame(loop);}loop();}
function drawTower(){let base=20,credits=100,wave=1,score=0,en=[{t:0},{t:120}],towers=[{x:260,y:120},{x:450,y:300}];function loop(){tick++;x.fillStyle='#111827';x.fillRect(0,0,760,420);t(scene.title,20);t('Base HP '+base+' Credits '+credits+' Wave '+wave+' Score '+score,40);x.strokeStyle='#64748b';x.beginPath();x.moveTo(50,210);x.lineTo(700,210);x.stroke();for(const s of towers){x.fillStyle='#60a5fa';x.fillRect(s.x,s.y,20,20);}for(const e of en){e.t+=0.7+wave*0.03;const ex=50+e.t,ey=210;x.fillStyle='#ef4444';x.fillRect(ex,ey,14,14);for(const s of towers){if(Math.abs(s.x-ex)<120&&tick%25===0){score+=10;e.t=0;credits+=5;}}if(ex>700){base--;e.t=0;}}if(tick%700===0){wave++;en.push({t:0});}requestAnimationFrame(loop);}loop();}
function drawBoss(){let p={x:120,y:210},hp=5,score=0,boss={x:620,y:170,hp:240},orbs=[{x:300,y:120}],shots=[];function loop(){tick++;x.fillStyle='#0b1020';x.fillRect(0,0,760,420);t(scene.title,20);t('HP '+hp+' Score '+score+' Boss HP '+boss.hp,40);if(K.ArrowUp)p.y-=3;if(K.ArrowDown)p.y+=3;if(K.ArrowLeft)p.x-=3;if(K.ArrowRight)p.x+=3;p.x=Math.max(0,Math.min(740,p.x));p.y=Math.max(0,Math.min(400,p.y));if(tick%20===0)shots.push({x:p.x+18,y:p.y+10});for(const s of shots){s.x+=5;if(Math.abs(s.x-boss.x)<30&&Math.abs(s.y-boss.y)<50){boss.hp-=2;score+=20;}}if(tick%40===0&&Math.abs(p.y-(40+Math.random()*340))<20)hp--;for(const o of orbs){if(Math.abs(p.x-o.x)<14&&Math.abs(p.y-o.y)<14){hp=Math.min(5,hp+1);o.x=120+Math.random()*520;o.y=40+Math.random()*330;}}x.fillStyle='#22d3ee';x.fillRect(p.x,p.y,20,20);x.fillStyle='#a78bfa';x.fillRect(boss.x,boss.y,40,80);if(boss.hp<=0)t('VICTORY',64);requestAnimationFrame(loop);}loop();}
function drawCard(){let php=30,ehp=30,turn=1;c.onclick=(e)=>{const r=c.getBoundingClientRect(),px=e.clientX-r.left,py=e.clientY-r.top;if(py>340&&py<390){if(px<230)ehp-=6;else if(px<470)php=Math.min(30,php+4);else ehp-=10;if(ehp>0)php-=4;turn++;}};function loop(){x.fillStyle='#1f2937';x.fillRect(0,0,760,420);t(scene.title,20);t('Player HP '+php+' Enemy HP '+ehp+' Turn '+turn,40);x.fillStyle='#334155';x.fillRect(20,340,200,50);x.fillRect(260,340,200,50);x.fillRect(500,340,220,50);x.fillStyle='#fff';x.fillText('Attack',95,370);x.fillText('Shield',340,370);x.fillText('Energy Blast',565,370);requestAnimationFrame(loop);}loop();}
function drawResource(){let energy=20,minerals=10,shield=30,level=1;c.onclick=(e)=>{const r=c.getBoundingClientRect(),px=e.clientX-r.left,py=e.clientY-r.top;if(py<330||py>390)return;if(px<250&&energy>=3){energy-=3;minerals+=5;}else if(px<500&&minerals>=8){minerals-=8;energy+=6;}else if(shield<40&&minerals>=5){minerals-=5;shield+=5;}if(minerals>30){level++;minerals-=12;}};function loop(){if(tick++%120===0){energy++;minerals+=2;if(Math.random()<0.2)shield-=2;}x.fillStyle='#0f172a';x.fillRect(0,0,760,420);t(scene.title,20);t('Energy '+energy+' Minerals '+minerals+' Shield HP '+shield+' Colony Level '+level,40);requestAnimationFrame(loop);}loop();}
const template = scene.template; if(template==='quest_arena_rpg')drawQuest(); else if(template==='boss_fight')drawBoss(); else if(template==='tower_defense')drawTower(); else if(template==='card_battle')drawCard(); else if(template==='resource_management')drawResource(); else drawQuest();</script></body></html>`;
}

export const buildPreviewHtml = (brief:Record<string,unknown>) => renderPreviewHtml(buildGameSceneConfig(brief as GameBrief));
export const buildAssets = (brief:Record<string,unknown>) => buildGameAssets(brief as GameBrief);

export function buildWeb3Package(project:GFProject, brief:Record<string,unknown>, assets:Record<string,unknown>[]){const b=brief as GameBrief;return{manifest:{project_id:project.id,title:b.title,target_chain:project.target_chain,generated_at:new Date().toISOString(),template:b.template,prompt:b.prompt},item_schema:{version:"1.0",items:assets.map((a)=>({type:a.asset_type,name:a.name,rarity:a.rarity})),recommended:b.web3.item_schema},nft_metadata:assets.map((a)=>({name:a.name,description:a.description,attributes:[{trait_type:"type",value:a.asset_type},{trait_type:"rarity",value:a.rarity},...b.web3.nft_traits.map((t)=>({trait_type:"tag",value:t}))]})),reward_config:{currency:"points",events:b.web3.reward_events},adapter_config:{chain:"arbitrum-sepolia",mode:"readiness-only",wallet_required:false,tx_signing:false,deploy:false}};}

export const gameFactoryDb={
 async createProject(input:{title?:string;prompt:string;genre?:string;visual_style?:string;target_chain?:string}){const q=`insert into game_factory_projects (title,prompt,genre,visual_style,target_chain) values ($1,$2,$3,$4,$5) returning id::text,title,prompt,genre,visual_style,target_chain,status,metadata,created_at::text,updated_at::text`;return (await web3Db.query<GFProject>(q,[input.title||null,input.prompt,input.genre||null,input.visual_style||null,input.target_chain||"arbitrum-sepolia"])).rows[0];},
 async listProjects(){return (await web3Db.query<GFProject>(`select id::text,title,prompt,genre,visual_style,target_chain,status,metadata,created_at::text,updated_at::text from game_factory_projects order by created_at desc`)).rows;},
 async getProject(id:string){return (await web3Db.query<GFProject>(`select id::text,title,prompt,genre,visual_style,target_chain,status,metadata,created_at::text,updated_at::text from game_factory_projects where id=$1 limit 1`,[id])).rows[0]??null;},
 async getBrief(id:string){return (await web3Db.query<{brief:Record<string,unknown>}>(`select brief from game_factory_briefs where project_id=$1 order by created_at desc limit 1`,[id])).rows[0]?.brief??null;},
 async getAssets(id:string){return (await web3Db.query<Record<string,unknown>>(`select asset_type,name,description,rarity,metadata from game_factory_assets where project_id=$1 order by created_at asc`,[id])).rows;},
 async getFiles(id:string){return (await web3Db.query<Record<string,unknown>>(`select file_path,file_type,content,metadata from game_factory_generated_files where project_id=$1 order by created_at asc`,[id])).rows;},
 async getWeb3Package(id:string){return (await web3Db.query<Record<string,unknown>>(`select manifest,item_schema,nft_metadata,reward_config,adapter_config from game_factory_web3_packages where project_id=$1 order by created_at desc limit 1`,[id])).rows[0]??null;}
};
