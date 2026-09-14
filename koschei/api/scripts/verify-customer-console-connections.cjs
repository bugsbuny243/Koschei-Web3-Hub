'use strict';
const fs=require('node:fs');
const path=require('node:path');
const assert=require('node:assert/strict');
const root=path.resolve(__dirname,'../public');
const files=[...fs.readdirSync(root).filter(f=>f.endsWith('.html')).map(f=>path.join(root,f)),...fs.readdirSync(path.join(root,'js')).filter(f=>f.endsWith('.js')).map(f=>path.join(root,'js',f))];
let checked=0;
for(const file of files){
 const source=fs.readFileSync(file,'utf8');
 for(const match of source.matchAll(/["'`]((?:\/js\/|\/css\/)[\w./-]+\.(?:js|css))(?:\?[^"'`]*)?["'`]/g)){
  assert.ok(fs.existsSync(path.join(root,match[1])),`Missing asset ${match[1]} referenced by ${file}`);checked++;
 }
}
for(const page of ['dashboard.html','scan.html']){
 const html=fs.readFileSync(path.join(root,page),'utf8');
 const ids=[...html.matchAll(/\bid="([^"]+)"/g)].map(m=>m[1]);
 assert.equal(new Set(ids).size,ids.length,`Duplicate form/result ID in ${page}`);
}
const html=fs.readFileSync(path.join(root,'scan.html'),'utf8');
assert.ok(html.indexOf('customer-scan-entry.js')<html.indexOf('public-solana-scan.js'),'Route contract must load before legacy scan bootstrap');
assert.match(html,/data-koschei-enhancement="universal-address-scan-v1"/,'Static controller must suppress duplicate dynamic loading');
assert.match(html,/id="advancedTools"/,'Existing investigation modes must remain reachable');
console.log(`Customer console wiring: ${checked} asset references resolved; entry, results and tool modes connected.`);
