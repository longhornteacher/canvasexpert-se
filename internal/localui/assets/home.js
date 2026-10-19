(()=>{
const text=document.querySelector("[data-prompt-text]"),copy=document.querySelector("[data-copy-prompt]");
if(!text||!copy)return;
const commands=[...document.querySelectorAll("[data-command]")],courses=[...document.querySelectorAll("[data-course]")];
const pressed=list=>list.find(b=>b.getAttribute("aria-pressed")==="true");
const draw=()=>{const c=pressed(commands)||commands[0],course=pressed(courses)?.dataset.course||"";text.textContent=c.dataset.prompt.split("{course}").join(course||"one of my courses");copy.textContent="Copy"};
const pick=(list,b)=>{list.forEach(x=>x.setAttribute("aria-pressed",x===b?"true":"false"));draw()};
commands.forEach(b=>b.addEventListener("click",()=>pick(commands,b)));
courses.forEach(b=>b.addEventListener("click",()=>pick(courses,b)));
const fallback=()=>{const r=document.createRange();r.selectNodeContents(text);const s=getSelection();s.removeAllRanges();s.addRange(r);return document.execCommand("copy")};
copy.addEventListener("click",()=>{const done=()=>{copy.textContent="Copied";setTimeout(()=>{copy.textContent="Copy"},1500)};
 (navigator.clipboard?navigator.clipboard.writeText(text.textContent):Promise.reject()).then(done,()=>{if(fallback())done();else copy.textContent="Select and copy"})});
})();
