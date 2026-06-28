(function(){
  function ready(fn){if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',fn,{once:true});}else{fn();}}
  ready(function(){
    var links=[['/dashboard','Dashboard'],['/security-radar','Radar'],['/token-2022-scanner','Token-2022'],['/transaction-firewall','Firewall'],['/watchlist','Watchlist'],['/webhooks','Webhooks'],['/pilot','Integrate'],['/pricing','Plans']];
    var current=(location.pathname||'/').replace(/\.html$/,'').replace(/\/$/,'')||'/';
    var existing=document.querySelector('.top .nav, header.top nav.nav, nav.top .nav');
    var nav=existing||document.createElement('nav');
    nav.className=(existing?'nav ':'')+'koschei-global-nav';
    nav.setAttribute('aria-label','Primary');
    while(nav.firstChild)nav.removeChild(nav.firstChild);
    links.forEach(function(item){var a=document.createElement('a');a.href=item[0];a.textContent=item[1];if(current===item[0])a.setAttribute('aria-current','page');nav.appendChild(a);});
    if(!existing){var top=document.querySelector('header.top,.top');if(top){nav.className+=' detached';top.parentNode.insertBefore(nav,top.nextSibling);}}
    var bottom=document.querySelector('nav.bottom');if(bottom)bottom.remove();
    if(!document.querySelector('.koschei-footer')){var footer=document.createElement('footer');footer.className='koschei-footer';footer.innerHTML='<span>Koschei ARVIS · Solana pre-signing risk infrastructure</span><span><a href="/architecture">Architecture</a> · <a href="/developers">Developers</a> · <a href="/pilot">Integration Pilot</a> · <a href="/pricing">Plans</a></span>';document.body.appendChild(footer);}
  });
})();