// Keeps unsaved edits in the current browser tab only (sessionStorage).
// The page declares its draft fields in data-draft-fields: names ending in "_"
// match as prefixes (active_ nick_), the rest match exactly (voice sync_root).
// A per-hub key keeps another local hub session from restoring private notes.
(()=>{
const root=document.querySelector("[data-draft-key]");
if(!root)return;
const spec=(root.dataset.draftFields||"").split(/\s+/).filter(Boolean);
const wanted=name=>spec.some(s=>s.endsWith("_")?name.startsWith(s):name===s);
const key=(root.dataset.draftKeyPrefix||"")+root.dataset.draftKey;
const fields=()=>Array.from(root.querySelectorAll("input[name],textarea[name],select[name]")).filter(e=>wanted(e.name));
let values={};
try{values=JSON.parse(sessionStorage.getItem(key)||"{}")}catch{}
const persist=()=>{try{if(Object.keys(values).length)sessionStorage.setItem(key,JSON.stringify(values));else sessionStorage.removeItem(key)}catch{}};
const cleanURL=(u,...params)=>{for(const p of params)u.searchParams.delete(p);history.replaceState(null,"",u.pathname+u.search+u.hash)};

// A saved form clears its own drafts: "true" clears every field on the page,
// a group name clears just that form's fields. The page declares its groups in
// data-draft-groups="name:field,field name:field".
const groups={};
for(const g of (root.dataset.draftGroups||"").split(/\s+/).filter(Boolean)){const[name,list]=g.split(":");groups[name]=(list||"").split(",").filter(Boolean)}
const clear=root.dataset.clearDraft;
if(clear&&clear!=="false"){
 const names=clear==="true"?fields().map(e=>e.name):(groups[clear]||[]);
 for(const n of names)delete values[n];
 persist();
 cleanURL(new URL(location.href),"saved","clear");
}

// A folder chosen with Browse arrives in the URL; it is kept as a draft but
// does not count as restored edits.
const url=new URL(location.href);
const fromURL=spec.includes("sync_root")&&url.searchParams.has("sync_root");
if(fromURL){
 values.sync_root=url.searchParams.get("sync_root");
 persist();
 cleanURL(url,"sync_root");
}

let restored=false;
for(const e of fields()){
 if(!Object.prototype.hasOwnProperty.call(values,e.name))continue;
 const v=values[e.name];
 const changed=e.type==="checkbox"?e.checked!==(v===true):e.value!==v;
 if(changed&&!(fromURL&&e.name==="sync_root"))restored=true;
 if(e.type==="checkbox")e.checked=v===true;else e.value=v;
}

// A restored draft overrides saved values, so say so and let the teacher drop
// the edits on this page. Drafts for other pages are kept.
const showNotice=()=>{
 const before=root.tagName==="FORM"?root:((root.querySelector("h1")||{}).nextSibling||root.firstChild);
 const names=fields().map(e=>e.name);
 const p=document.createElement("p");
 p.className="draft-restored";
 p.setAttribute("role","status");
 p.textContent="Unsaved edits from this tab are shown. Saving replaces the saved values. ";
 const b=document.createElement("button");
 b.type="button";
 b.className="small secondary";
 b.textContent="Discard unsaved edits";
 b.addEventListener("click",()=>{for(const n of names)delete values[n];persist();location.reload()});
 p.append(b);
 before.parentNode.insertBefore(p,before);
};
if(restored)showNotice();

const save=event=>{
 const e=event.target;
 if(!e.name||!wanted(e.name))return;
 values[e.name]=e.type==="checkbox"?e.checked:e.value;
 try{sessionStorage.setItem(key,JSON.stringify(values))}catch{}
};
root.addEventListener("input",save);
root.addEventListener("change",save);
})();
