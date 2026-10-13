(()=>{
const panel=document.querySelector("[data-canvas-health]");
if(!panel||panel.dataset.state!=="Checking…")return;
const state=panel.querySelector("[data-health-state]");
const note=panel.querySelector("[data-health-note]");
const mark=panel.querySelector("[data-health-mark]");
const buttons=document.querySelectorAll("[data-canvas-test]");
const setButton=(checking)=>buttons.forEach(b=>{b.disabled=checking;b.textContent=checking?"Checking…":"Test"});
const deadline=Date.now()+15000;let finished=false;
const finishUnavailable=()=>{if(finished)return;finished=true;panel.dataset.state="Unavailable";state.textContent="Unavailable";note.textContent="The local page could not read the check result. Reload to retry.";mark.className="status-mark bad";setButton(false)};
setButton(true);
const poll=()=>fetch("/_cse/canvas-health",{cache:"no-store",headers:{Accept:"application/json"}})
 .then(r=>{if(!r.ok)throw new Error("status");return r.json()})
 .then(s=>{panel.dataset.state=s.state;state.textContent=s.state;note.textContent=s.note;mark.className="status-mark "+(s.state==="Connected"?"good":s.state==="Token rejected"||s.state==="Unavailable"?"bad":s.state==="Checking…"?"waiting":"");const account=panel.querySelector("[data-health-account]"),accountRow=panel.querySelector("[data-health-account-row]");if(account&&accountRow){account.textContent=s.account||"";accountRow.hidden=!s.account}if(s.state==="Checking…"){if(Date.now()<deadline)setTimeout(poll,250);else finishUnavailable()}else{finished=true;setButton(false)}})
 .catch(()=>{if(Date.now()<deadline)setTimeout(poll,250);else finishUnavailable()});
setTimeout(poll,150);
})();
