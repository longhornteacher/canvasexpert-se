// Private contact drafts stay in this browser tab; no network requests.
(()=>{
const form=document.querySelector('form[action="/students"]');
if(!form)return;
const key=form.dataset.draftKey?`contacts:${form.dataset.draftKey}`:null;
let drafts={};
if(key){try{drafts=JSON.parse(sessionStorage.getItem(key)||'{}');if(form.dataset.clearDraft==='true'){drafts={};sessionStorage.removeItem(key)}}catch{}}
const sections=Array.from(form.querySelectorAll('[data-contact-ref]'));
const rows=s=>Array.from(s.querySelectorAll('.parent-contact'));
const renumber=s=>rows(s).forEach((r,i)=>r.querySelectorAll('input[type="radio"]').forEach(e=>e.name=`${e.name.startsWith('contact_send_email_')?'contact_send_email_':'contact_text_'}${s.dataset.contactRef}_${i}`));
const hint=r=>{
 const phone=r.querySelector('[data-contact-phone]').value.trim();
 const email=r.querySelector('input[type="email"]').value.trim();
 const selected=Array.from(r.querySelectorAll('input[type="radio"]:checked'));
 r.querySelector('[data-contact-hint]').textContent=[selected.some(e=>e.name.startsWith('contact_text_')&&e.value==='yes')&&!phone?'Add a phone number to enable texting.':'',selected.some(e=>e.name.startsWith('contact_send_email_')&&e.value==='yes')&&!email?'Add an email address to enable email.':''].filter(Boolean).join(' ');
};
const snapshot=s=>({email:s.querySelector('input[name^="student_email_"]').value,rows:rows(s).map(r=>({phone:r.querySelector('[data-contact-phone]').value,email:r.querySelector('input[type="email"]').value,text:r.querySelector('input[name^="contact_text_"]:checked')?.value||'no',send:r.querySelector('input[name^="contact_send_email_"]:checked')?.value||'no'}))});
const persist=s=>{if(!key)return;drafts[s.dataset.contactRef]=snapshot(s);try{sessionStorage.setItem(key,JSON.stringify(drafts))}catch{}};
for(const s of sections){
 const prototype=rows(s)[0].cloneNode(true);
 const blank=()=>{const r=prototype.cloneNode(true);r.querySelectorAll('input').forEach(e=>{if(e.type==='radio')e.checked=e.value==='no';else e.value=''});r.querySelectorAll('input[type="radio"]').forEach(e=>e.name=`${e.name.startsWith('contact_send_email_')?'contact_send_email_':'contact_text_'}${s.dataset.contactRef}_${rows(s).length}`);return r};
 const saved=drafts[s.dataset.contactRef];
 if(saved){
  s.querySelector('input[name^="student_email_"]').value=saved.email;
  s.querySelector('[data-contact-rows]').replaceChildren();
  for(const item of saved.rows){const r=blank();s.querySelector('[data-contact-rows]').append(r);renumber(s);r.querySelector('[data-contact-phone]').value=item.phone;r.querySelector('input[type="email"]').value=item.email;r.querySelectorAll('input[type="radio"]').forEach(e=>e.checked=e.value===(e.name.startsWith('contact_text_')?item.text:item.send))}
  s.open=true;
 }
 s.addEventListener('click',e=>{
  if(e.target.closest('[data-add-contact]')){s.querySelector('[data-contact-rows]').append(blank());renumber(s);persist(s)}
  const remove=e.target.closest('[data-remove-contact]');
  if(remove){remove.closest('.parent-contact').remove();renumber(s);persist(s)}
 });
 s.addEventListener('input',e=>{const r=e.target.closest('.parent-contact');if(r)hint(r);persist(s)});
 s.addEventListener('change',e=>{const r=e.target.closest('.parent-contact');if(r)hint(r);persist(s)});
 s.addEventListener('focusout',e=>{
  if(e.target.matches('[data-contact-phone]')){let digits=e.target.value.replace(/\D/g,'');if(digits.length===11&&digits.startsWith('1'))digits=digits.slice(1);if(digits.length===10&&/^[\d\s()+.\-]*$/.test(e.target.value))e.target.value=`${digits.slice(0,3)}-${digits.slice(3,6)}-${digits.slice(6)}`}
  if(e.target.type==='email')e.target.value=e.target.value.trim();
  persist(s);
 });
 rows(s).forEach(hint);
}
if(Object.keys(drafts).length){const p=document.createElement('p');p.className='draft-restored';p.textContent='Unsaved contact details from this tab are shown. ';const b=document.createElement('button');b.type='button';b.textContent='Discard unsaved contact details';b.onclick=()=>{sessionStorage.removeItem(key);location.reload()};p.append(b);form.prepend(p)}
})();
