(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerAuthoritySourceAddressesInstalled)return;
  window.__ownerAuthoritySourceAddressesInstalled=true;

  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const rawAddress=value=>typeof value==='string'?value.trim():'';
  const isPubkey=value=>/^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(value);
  const rootFor=value=>typeof value==='string'?document.getElementById(value):value;

  function sourceFor(payload){
    const envelope=obj(payload),report=obj(envelope.investigation_report||envelope.report||envelope);
    const candidates=[
      obj(report.legacy_14_arm_radar).source_context,
      report.source_context,
      obj(envelope.legacy_14_arm_radar).source_context,
      envelope.source_context,
    ];
    const target=rawAddress(report.target||envelope.target);
    for(const item of candidates){
      const source=obj(item);
      if(Object.prototype.hasOwnProperty.call(source,'mint_authority')||
         Object.prototype.hasOwnProperty.call(source,'freeze_authority'))return source;
      // Production creator resolution retains mint/freeze authority fields
      // *inside* creator_resolution, not on source_context itself.
      const resolution=obj(source.creator_resolution);
      if(!target||rawAddress(resolution.address)!==target)continue;
      if(Object.prototype.hasOwnProperty.call(resolution,'mint_authority')||
         Object.prototype.hasOwnProperty.call(resolution,'freeze_authority')){
        return {
          mint_authority:resolution.mint_authority,
          freeze_authority:resolution.freeze_authority,
          limitations:Array.isArray(resolution.limitations)?resolution.limitations:source.limitations,
        };
      }
    }
    return null;
  }
  function line(parent,label,address){
    const row=document.createElement('div');
    row.className='owner-authority-source-row';
    const title=document.createElement('strong');
    title.textContent=label;
    const value=document.createElement('code');
    value.textContent=isPubkey(address)?address:'Kaynakta doğrulanabilir adres yok; yetki kapalı/revoked sonucu çıkarılamaz.';
    row.appendChild(title);
    row.appendChild(value);
    parent.appendChild(row);
  }
  function mount(root,payload){
    root=rootFor(root);
    if(!root)return;
    const card=root.querySelector('#target-identity-card')||root.querySelector('#verdict-card');
    if(!card)return;
    card.querySelector('.owner-authority-source')?.remove();
    const source=sourceFor(payload);
    if(!source)return;
    const mint=rawAddress(source.mint_authority),freeze=rawAddress(source.freeze_authority);
    if(!isPubkey(mint)&&!isPubkey(freeze))return;
    const section=document.createElement('details');
    section.className='owner-authority-source';
    const title=document.createElement('summary');
    title.textContent='Yetki adresleri · kaynak beyanı, bağımsız RPC doğrulaması değil';
    section.appendChild(title);
    line(section,'Mint authority',mint);
    line(section,'Freeze authority',freeze);
    const limit=document.createElement('p');
    limit.className='muted';
    limit.textContent='Bu adresler kaynak bağlamından alınmıştır; tek başına güncel kanonik yetki veya kötü niyet kanıtı değildir. Creator discovery, canonical on-chain verification yerine geçmez.';
    section.appendChild(limit);
    const limits=Array.isArray(source.limitations)?source.limitations.filter(v=>typeof v==='string'&&v.trim()).slice(0,2):[];
    for(const value of limits){
      const note=document.createElement('p');
      note.className='muted';
      note.textContent=value;
      section.appendChild(note);
    }
    card.appendChild(section);
  }
  function style(){
    if(document.getElementById('ownerAuthoritySourceStyles'))return;
    const node=document.createElement('style');
    node.id='ownerAuthoritySourceStyles';
    node.textContent='.owner-authority-source{margin:12px 0;padding:12px;border:1px solid #ffffff29;border-radius:11px}.owner-authority-source summary{cursor:pointer;font-weight:700}.owner-authority-source-row{display:grid;gap:4px;margin:9px 0}.owner-authority-source-row code{display:block;overflow-wrap:anywhere;word-break:break-word;user-select:text;font-size:12px}.owner-authority-source p{font-size:12px;line-height:1.5}';
    document.head.appendChild(node);
  }
  const originalRender=kit.renderUnified;
  const originalScan=kit.scan;
  const renderUnified=(root,payload)=>{
    const result=originalRender?.(root,payload);
    style();mount(root,payload);return result;
  };
  const scan=async(target,root)=>{
    const payload=await originalScan(target,root);
    style();mount(root,payload);return payload;
  };
  window.OwnerRadarKit={...kit,renderUnified,scan};
})();
