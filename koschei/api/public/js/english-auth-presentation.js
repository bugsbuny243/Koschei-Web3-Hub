// Runtime-injected presentation adapter for the frozen authentication surfaces.
(()=>{
'use strict';
if(window.__koscheiAuthEnglishOverlayInstalled)return;
window.__koscheiAuthEnglishOverlayInstalled=true;
document.documentElement.lang='en';

const TOKEN_KEY='koschei_jwt';
const LEGACY_TOKEN_KEY='koschei_token';
const nativeFetch=window.fetch.bind(window);

function isJWT(value){
  if(!value||typeof value!=='string')return false;
  const parts=value.split('.');
  return parts.length===3&&parts.every(Boolean);
}

function findJWT(value){
  if(!value||typeof value!=='object')return '';
  const candidates=[
    value.token,
    value.jwt,
    value.access_token,
    value.id_token,
    value.auth_token,
    value.data&&value.data.token,
    value.data&&value.data.jwt,
    value.data&&value.data.access_token,
    value.data&&value.data.id_token,
    value.session&&value.session.token,
    value.session&&value.session.jwt,
    value.session&&value.session.access_token,
    value.session&&value.session.id_token
  ];
  return candidates.find(isJWT)||'';
}

async function readJSON(response){
  const text=await response.text().catch(()=> '');
  if(!text)return {};
  try{return JSON.parse(text);}catch{return {message:text};}
}

function saveJWT(token){
  if(!isJWT(token))return;
  try{
    localStorage.setItem(TOKEN_KEY,token);
    localStorage.setItem(LEGACY_TOKEN_KEY,token);
  }catch{}
}

function clearJWT(){
  try{
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(LEGACY_TOKEN_KEY);
  }catch{}
}

function authError(data,fallback){
  const raw=String((data&&(data.message||data.error_description||data.error||data.detail))||'').trim();
  if(!raw)return fallback;
  const normalized=raw.toLowerCase();
  if(normalized.includes('invalid email or password')||normalized.includes('invalid credentials'))return 'Invalid email or password.';
  if(normalized.includes('auth_session_missing')||normalized.includes('session token'))return 'The authentication provider did not return a session. Please try again.';
  if(normalized.includes('temporarily unreachable')||normalized.includes('unavailable'))return 'Authentication is temporarily unavailable. Please try again.';
  return raw;
}

async function sameOriginEmailAuth(path,email,password,includeName){
  const payload={
    email:String(email||'').trim(),
    password:String(password||''),
    callbackURL:window.location.origin.replace(/\/+$/,'')+'/dashboard'
  };
  if(includeName){
    payload.name=(payload.email.split('@')[0]||'User').trim()||'User';
  }
  const response=await nativeFetch(path,{
    method:'POST',
    credentials:'same-origin',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify(payload)
  });
  const data=await readJSON(response);
  if(!response.ok)throw new Error(authError(data,includeName?'Account creation failed.':'Sign-in failed.'));
  const token=findJWT(data);
  if(!token)throw new Error('The authentication provider did not return a session. Please try again.');
  saveJWT(token);
  const meResponse=await nativeFetch('/api/me',{
    method:'GET',
    credentials:'same-origin',
    headers:{Authorization:'Bearer '+token}
  });
  const me=await readJSON(meResponse);
  if(!meResponse.ok){
    clearJWT();
    throw new Error(authError(me,'The authenticated session could not be verified.'));
  }
  return {...data,me,access_token:token,token_type:'Bearer'};
}

function installSameOriginAuthContract(){
  const auth=window.KoscheiAuth;
  if(!auth||window.__koscheiSameOriginEmailAuthInstalled)return false;
  window.__koscheiSameOriginEmailAuthInstalled=true;
  auth.signIn=(email,password)=>sameOriginEmailAuth('/api/auth/login',email,password,false);
  auth.signUp=(email,password)=>sameOriginEmailAuth('/api/auth/register',email,password,true);
  return true;
}

// The frozen auth helper may start a provider-session restore while /api/config
// is resolving. Block only those cross-origin restore probes; email/password
// authentication is always handled through the same-origin backend contract.
window.fetch=async function(input,init){
  try{
    const raw=typeof input==='string'?input:(input&&input.url)||'';
    const target=new URL(raw,window.location.origin);
    const crossOrigin=target.origin!==window.location.origin;
    const providerHost=/neonauth\.|\.neon\.tech$/i.test(target.hostname);
    const providerSessionPath=/(?:\/token|\/get-session)$/i.test(target.pathname);
    if(crossOrigin&&providerHost&&providerSessionPath){
      return new Response('{}',{status:404,headers:{'Content-Type':'application/json'}});
    }
  }catch{}
  return nativeFetch(input,init);
};

installSameOriginAuthContract();
queueMicrotask(installSameOriginAuthContract);
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',installSameOriginAuthContract,{once:true});

const exact=new Map(Object.entries({
  'Giriş Yap':'Sign In',
  'Hesap Oluştur':'Create Account',
  'E-posta':'Email',
  'Şifre':'Password',
  'Şifreyi onayla':'Confirm password',
  'Şifremi unuttum':'Forgot password?',
  'Hesabınız yok mu?':'No account yet?',
  'Hesap oluştur':'Create an account',
  'Zaten hesabınız var mı?':'Already have an account?',
  'Hesap oturumunu aç. Derin ARVIS araçları için girişten sonra Phantom cüzdanını doğrula ve KOSCH holder durumunu kontrol et.':'Open your account session to manage your Koschei SaaS entitlement and access features included in your active plan. Token holdings and wallet balances do not unlock paid access.',
  'Hesap oturumunu aç. Ücretli ARVIS araçları aktif Koschei SaaS aboneliğine göre açılır.':'Continue to the customer security command center and your account-scoped intelligence workspace.',
  'Hesap ücretsiz oluşturulur. Public Safe Check açıktır; derin ürün erişimi doğrulanmış KOSCH bakiyesiyle açılır.':'Account creation is free. Public Safe Check remains open; paid product features unlock through an active SaaS entitlement after Paddle checkout.',
  'Hesap ücretsiz oluşturulur. Public Safe Check açıktır; ücretli özellikler yalnızca aktif Koschei SaaS aboneliğiyle açılır.':'Create an account for the Koschei Web3 customer workspace. Paid capabilities remain server-authoritative SaaS entitlements.',
  '✓ Ücretsiz hesap için kart bilgisi gerekmez':'✓ No payment method is required to create an account',
  '✓ Ücretli planlar standart SaaS aboneliği olarak yönetilir':'✓ Paid plans are managed as standard SaaS entitlements',
  '✓ Plan erişimi aktif abonelik yetkisine göre belirlenir':'✓ Product access follows the active server-authoritative entitlement',
  '✓ Paket veya kart bilgisi gerekmez':'✓ No payment method is required to create an account',
  '✓ Phantom yalnız mesaj imzalar':'✓ Phantom verification is identity-only',
  "✓ KOSCH bakiyesi ürün tier'ını otomatik açar":'✓ Token holdings do not unlock paid access',
  'Giriş başarılı.':'Sign-in successful.',
  'Giriş yapılamadı.':'Sign-in failed.',
  'Geçerli e-posta girin.':'Enter a valid email address.',
  'Şifre en az 8 karakter olmalı.':'Password must contain at least 8 characters.',
  'Şifreler eşleşmiyor.':'Passwords do not match.',
  'Hesap oluşturuldu.':'Account created.',
  'Hesap oluşturulamadı.':'Account creation failed.'
}));

function translateText(value){
  const source=String(value||'');
  const trimmed=source.trim();
  if(!trimmed)return source;
  let translated=exact.get(trimmed);
  if(!translated){
    translated=trimmed
      .replace(/^Giriş Yap\s*[—-]\s*/,'Sign In — ')
      .replace(/^Hesap Oluştur\s*[—-]\s*/,'Create Account — ')
      .replace(/Koschei ARVIS hesabınıza Neon Auth ile giriş yapın; derin araçlar doğrulanmış KOSCH holder access gerektirir\./g,'Sign in to Koschei ARVIS with Neon Auth. Paid features require an active SaaS entitlement.')
      .replace(/Koschei ARVIS hesabınızı Neon Auth ile oluşturun; derin araçlar doğrulanmış KOSCH holder access ile açılır\./g,'Create a Koschei ARVIS account with Neon Auth. Paid features unlock after verified Paddle billing.');
  }
  if(translated===trimmed&&translated===source)return source;
  const leading=source.match(/^\s*/)?.[0]||'';
  const trailing=source.match(/\s*$/)?.[0]||'';
  return leading+translated+trailing;
}

function visit(node){
  if(node.nodeType===Node.TEXT_NODE){
    const parent=node.parentElement;
    if(!parent||['SCRIPT','STYLE','CODE','PRE'].includes(parent.tagName))return;
    const translated=translateText(node.nodeValue);
    if(translated!==node.nodeValue)node.nodeValue=translated;
    return;
  }
  if(node.nodeType!==Node.ELEMENT_NODE)return;
  const element=node;
  if(element.hasAttribute('placeholder'))element.setAttribute('placeholder',translateText(element.getAttribute('placeholder')));
  if(element.tagName==='TITLE')element.textContent=translateText(element.textContent);
  for(const child of element.childNodes)visit(child);
}


function installSecureAccessStyle(){
  if(document.getElementById('koschei-secure-access-style'))return;
  const style=document.createElement('style');
  style.id='koschei-secure-access-style';
  style.textContent='body{display:block!important;align-items:initial!important;justify-content:initial!important;min-width:320px!important;background:radial-gradient(circle at 10% 8%,rgba(46,144,120,.12),transparent 28rem),radial-gradient(circle at 92% 90%,rgba(76,92,160,.1),transparent 30rem),linear-gradient(180deg,#080a0d,#07090c)!important;color:#f4f7f9!important}.blobs{display:none!important}.koschei-auth-shell{min-height:100vh;display:grid;grid-template-columns:minmax(0,1.08fr) minmax(420px,.92fr)}.koschei-auth-story{padding:clamp(28px,5vw,72px);display:flex;flex-direction:column;justify-content:space-between;border-right:1px solid rgba(255,255,255,.055)}.koschei-auth-brand{display:flex;align-items:center;gap:11px;width:max-content;color:#f4f7f9;text-decoration:none}.koschei-auth-brand-mark{width:40px;height:40px;display:grid;place-items:center;border:1px solid #34404d;border-radius:11px;background:linear-gradient(145deg,#171d25,#0d1116);color:#a7e9ff;font:900 18px SFMono-Regular,Consolas,monospace}.koschei-auth-brand-copy{display:grid;gap:2px}.koschei-auth-brand-copy strong{font-size:13px}.koschei-auth-brand-copy small{color:#6f7a87;font:700 9px SFMono-Regular,Consolas,monospace;letter-spacing:.1em;text-transform:uppercase}.koschei-auth-story-copy{max-width:720px;margin:auto 0}.koschei-auth-eyebrow{color:#72dfb6;font:800 10px SFMono-Regular,Consolas,monospace;letter-spacing:.13em;text-transform:uppercase}.koschei-auth-story-copy h1{margin:15px 0 0;font-size:clamp(48px,6vw,86px);line-height:.95;letter-spacing:-.055em}.koschei-auth-story-copy h1 span{display:block;color:#8796a5;font-weight:620}.koschei-auth-story-copy p{max-width:660px;margin:24px 0 0;color:#98a5b2;font-size:15px;line-height:1.75}.koschei-auth-principles{display:grid;grid-template-columns:repeat(3,1fr);gap:9px;margin-top:28px;max-width:700px}.koschei-auth-principles div{padding:13px;border:1px solid #242e38;border-radius:11px;background:#0b1015;display:grid;gap:4px}.koschei-auth-principles b{font:800 9px SFMono-Regular,Consolas,monospace;letter-spacing:.08em;color:#c7d6df}.koschei-auth-principles span{font-size:10px;color:#6f7d89}.koschei-auth-story-foot{color:#596672;font:700 9px SFMono-Regular,Consolas,monospace;letter-spacing:.08em;text-transform:uppercase}.koschei-auth-panel{display:grid;place-items:center;padding:28px}.koschei-auth-shell .card{position:relative!important;width:min(460px,100%)!important;max-width:460px!important;margin:0!important;padding:30px!important;border:1px solid #28323d!important;border-radius:20px!important;background:linear-gradient(180deg,rgba(17,22,29,.98),rgba(12,16,21,.99))!important;box-shadow:0 30px 90px rgba(0,0,0,.34),inset 0 1px rgba(255,255,255,.035)!important}.koschei-auth-shell .logo,.koschei-auth-shell .title{text-align:left!important}.koschei-auth-shell .logo{font-size:13px!important;color:#f4f7f9!important;letter-spacing:.02em}.koschei-auth-shell .pill{display:inline-flex!important;margin:10px 0 2px!important;padding:7px 9px!important;border:1px solid rgba(105,216,251,.22)!important;border-radius:999px!important;color:#a3e8fb!important;background:rgba(105,216,251,.035)!important;font:800 8px SFMono-Regular,Consolas,monospace!important;letter-spacing:.09em!important;text-transform:uppercase}.koschei-auth-shell .title{margin:14px 0 0!important;font-size:31px!important;letter-spacing:-.035em!important}.koschei-auth-shell .sub{margin:8px 0 22px!important;color:#84919e!important;text-align:left!important;font-size:13px!important;line-height:1.6!important}.koschei-auth-shell .form{display:grid!important;gap:15px!important}.koschei-auth-shell .form-group{display:grid!important;gap:8px!important}.koschei-auth-shell .form-label{font-size:12px!important;font-weight:750!important;color:#dce5ea!important}.koschei-auth-shell .form-input{width:100%!important;min-height:48px!important;padding:12px 13px!important;border:1px solid #323d48!important;border-radius:10px!important;background:#090e13!important;color:#f4f7f9!important;font-size:16px!important;outline:0!important}.koschei-auth-shell .form-input:focus{border-color:#5db9d4!important;box-shadow:0 0 0 3px rgba(105,216,251,.07)!important}.koschei-auth-shell .forgot{text-align:right!important;margin-top:-3px!important}.koschei-auth-shell .forgot a,.koschei-auth-shell .link a{color:#9fe7ff!important}.koschei-auth-shell .btn{width:100%!important;min-height:48px!important;padding:12px!important;border:1px solid #8adfff!important;border-radius:10px!important;background:linear-gradient(180deg,#9fe8ff,#5ed4fb)!important;color:#061017!important;font-size:14px!important;font-weight:800!important}.koschei-auth-shell .benefits{margin-top:2px!important;padding:12px!important;border:1px solid #26353d!important;border-radius:10px!important;background:#0b1115!important;color:#94a9b5!important;font-size:11px!important}.koschei-auth-shell .link{text-align:center!important;margin-top:17px!important;color:#7f8c98!important;font-size:12px!important}.koschei-access-boundary{margin-top:18px;padding:12px;border:1px solid #242d36;border-radius:10px;background:#0b0f14;color:#6f7e8b;font-size:10px;line-height:1.65}.koschei-access-boundary b{color:#a8b8c4}@media(max-width:900px){.koschei-auth-shell{grid-template-columns:1fr}.koschei-auth-story{min-height:auto;padding:22px 20px 14px;border-right:0;border-bottom:1px solid rgba(255,255,255,.055)}.koschei-auth-story-copy{margin:34px 0 0}.koschei-auth-story-copy h1{font-size:42px}.koschei-auth-story-copy p{font-size:13px;margin-top:15px}.koschei-auth-principles,.koschei-auth-story-foot{display:none}.koschei-auth-panel{padding:22px 16px 40px}.koschei-auth-shell .card{padding:24px!important}.koschei-auth-shell .title{font-size:27px!important}}@media(max-width:480px){.koschei-auth-brand-mark{width:36px;height:36px}.koschei-auth-story-copy h1{font-size:37px}.koschei-auth-story{padding:18px 16px 10px}.koschei-auth-panel{padding:16px 12px 32px}.koschei-auth-shell .card{padding:21px!important;border-radius:16px!important}}';
  document.head.appendChild(style);
}

function installSecureAccessPresentation(){
  if(!document.body||document.body.dataset.koscheiSecureAccess==='1')return;
  const loginForm=document.getElementById('loginForm');
  const registerForm=document.getElementById('registerForm');
  const form=loginForm||registerForm;
  const card=form?.closest('.card');
  if(!form||!card)return;

  document.body.dataset.koscheiSecureAccess='1';
  installSecureAccessStyle();
  const isLogin=Boolean(loginForm);
  document.title=isLogin?'Sign In · Koschei Web3':'Create Account · Koschei Web3';

  const logo=card.querySelector('.logo');
  const pill=card.querySelector('.pill');
  const title=card.querySelector('.title');
  const sub=card.querySelector('.sub');
  const submit=card.querySelector('#submitBtn');
  if(logo)logo.textContent='Koschei Web3';
  if(pill)pill.textContent=isLogin?'Secure Access':'Secure Account';
  if(title)title.textContent=isLogin?'Sign In':'Create Account';
  if(sub)sub.textContent=isLogin
    ?'Continue to the customer security command center and your account-scoped intelligence workspace.'
    :'Create an account for the Koschei Web3 customer workspace. Paid capabilities remain server-authoritative SaaS entitlements.';
  if(submit)submit.textContent=isLogin?'Open command center':'Create secure account';

  if(!card.querySelector('.koschei-access-boundary')){
    const boundary=document.createElement('div');
    boundary.className='koschei-access-boundary';
    const strong=document.createElement('b');
    strong.textContent='Access contract: ';
    boundary.append(strong,document.createTextNode('authentication changes access, never evidence, verdict or confidence. Wallet balances and token holdings do not silently unlock paid features.'));
    const link=card.querySelector('.link');
    if(link)card.insertBefore(boundary,link);else card.appendChild(boundary);
  }

  const shell=document.createElement('div');
  shell.className='koschei-auth-shell';
  const story=document.createElement('section');
  story.className='koschei-auth-story';
  story.setAttribute('aria-label','Koschei Web3 secure customer access');
  story.innerHTML='<a class="koschei-auth-brand" href="/" aria-label="Koschei Web3 home"><span class="koschei-auth-brand-mark">K</span><span class="koschei-auth-brand-copy"><strong>Koschei Web3</strong><small>Security Intelligence</small></span></a><div class="koschei-auth-story-copy"><span class="koschei-auth-eyebrow">Secure customer access</span><h1>Enter the intelligence workspace.<span>Evidence boundaries stay intact.</span></h1><p>Access account-scoped investigations, alerts, watchlists, professional tools and ARVIS workflows without changing the evidence model.</p><div class="koschei-auth-principles"><div><b>READ-ONLY</b><span>No custody or private keys</span></div><div><b>ARVIS</b><span>Evidence-backed verdict authority</span></div><div><b>FAIL-CLOSED</b><span>Unknown remains unknown</span></div></div></div><div class="koschei-auth-story-foot">Koschei Web3 · Customer Security Command Center</div>';
  const panel=document.createElement('section');
  panel.className='koschei-auth-panel';
  panel.appendChild(card);
  shell.append(story,panel);
  document.body.appendChild(shell);
}

function run(){
  installSameOriginAuthContract();
  document.title=translateText(document.title);
  document.querySelectorAll('meta[name="description"]').forEach(meta=>meta.setAttribute('content',translateText(meta.getAttribute('content'))));
  if(document.body){
    visit(document.body);
    installSecureAccessPresentation();
  }
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',run);else run();
new MutationObserver(records=>{for(const record of records){if(record.type==='characterData')visit(record.target);for(const node of record.addedNodes)visit(node);}}).observe(document.documentElement,{subtree:true,childList:true,characterData:true});
})();