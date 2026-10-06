(()=>{
'use strict';
if(window.__koscheiOwnerLogin)return;
window.__koscheiOwnerLogin=true;

const form=document.getElementById('ownerLoginForm');
const wallet=document.getElementById('ownerLoginWallet');
const secret=document.getElementById('ownerLoginSecret');
const button=document.getElementById('ownerLoginButton');
const errorBox=document.getElementById('ownerLoginError');

function showError(message){
  if(!errorBox)return;
  errorBox.textContent=String(message||'Owner login failed.');
  errorBox.hidden=false;
}
function clearError(){if(errorBox){errorBox.hidden=true;errorBox.textContent='';}}
async function request(path,options={}){
  const response=await fetch(path,{credentials:'same-origin',cache:'no-store',...options,headers:{'Content-Type':'application/json',...(options.headers||{})}});
  let data={};
  try{data=await response.json();}catch{}
  return {response,data};
}
async function redirectIfSessionExists(){
  try{
    const response=await fetch('/api/owner/operations',{method:'GET',credentials:'same-origin',cache:'no-store'});
    if(response.ok)location.replace('/owner-production');
  }catch{}
}

form?.addEventListener('submit',async event=>{
  event.preventDefault();
  clearError();
  const ownerSecret=secret?.value||'';
  if(!ownerSecret){showError('Owner secret is required.');secret?.focus();return;}
  if(button){button.disabled=true;button.textContent='Verifying…';}
  try{
    const {response,data}=await request('/api/owner/login',{method:'POST',body:JSON.stringify({wallet:(wallet?.value||'').trim(),secret:ownerSecret})});
    if(!response.ok){
      showError(response.status===404?'Owner credentials were not accepted.':(data?.error||`Owner login failed (${response.status}).`));
      return;
    }
    if(secret)secret.value='';
    location.replace('/owner-production');
  }catch{
    showError('Owner login service is unavailable.');
  }finally{
    if(button){button.disabled=false;button.textContent='Enter Owner Control Center';}
  }
});

redirectIfSessionExists();
})();
