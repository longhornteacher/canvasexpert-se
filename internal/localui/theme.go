package localui

// SEExpansion keeps the CLI's edition label consistent with the WebUI.
func SEExpansion() string { return "Special Edition" }

// funkyCSS is the shared look for the loopback pages. Pure CSS (the pages run
// under a no-script content security policy), so every effect is animation or
// transform only and honours prefers-reduced-motion.
// hubNav is the navigation bar shown on every page when served by the hub.
const hubNav = `<nav aria-label="Main navigation"><a href="/">Home</a><a href="/how">How it works</a><a href="/setup">Canvas</a><a href="/courses">Courses</a><a href="/students">Students</a><a href="/settings">Settings</a></nav>`

const funkyCSS = `
:root{--ink:#1b0b3a;--pink:#ff3ea5;--lime:#c6ff3d;--cyan:#27e1ff;--sun:#ffd23f;--grape:#7a3cff}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;padding:2.5rem 1rem 4rem;color:var(--ink);font:700 17px/1.45 "Trebuchet MS","Comic Sans MS",system-ui,sans-serif;
background:radial-gradient(circle at 12% 18%,var(--cyan) 0 9%,transparent 9.2%),radial-gradient(circle at 88% 82%,var(--pink) 0 11%,transparent 11.2%),
repeating-linear-gradient(135deg,var(--sun) 0 26px,#ffe680 26px 52px);background-size:auto,auto,200% 200%;animation:slide 14s linear infinite}
@keyframes slide{to{background-position:0 0,0 0,100% 100%}}
@keyframes wobble{0%,100%{transform:rotate(-2.2deg) skew(-3deg)}50%{transform:rotate(1.8deg) skew(3deg)}}
@keyframes bob{0%,100%{transform:translateY(0) rotate(-1deg)}50%{transform:translateY(-7px) rotate(1deg)}}
@keyframes hue{to{filter:hue-rotate(360deg)}}
main{max-width:38rem;margin:0 auto;background:#fff;border:5px solid var(--ink);border-radius:2rem 0.6rem 2.4rem 0.8rem;padding:2rem 2rem 2.2rem;box-shadow:12px 12px 0 var(--ink),24px 24px 0 var(--pink);transform:rotate(-.7deg);position:relative}
main.wide{max-width:78rem}
.badge{position:absolute;top:-1.6rem;right:1.4rem;background:var(--lime);border:4px solid var(--ink);padding:.15rem .8rem;border-radius:2rem;font-size:.85rem;transform:rotate(4deg);animation:bob 3s ease-in-out infinite}
h1{margin:.2rem 0 1rem;font-size:2.3rem;line-height:1.05;letter-spacing:-.03em;text-transform:uppercase;display:inline-block;background:linear-gradient(90deg,var(--pink),var(--grape),var(--cyan),var(--pink));background-size:300% 100%;-webkit-background-clip:text;background-clip:text;color:transparent;-webkit-text-stroke:2px var(--ink);animation:wobble 4s ease-in-out infinite,hue 9s linear infinite}
p{margin:.6rem 0}
label{display:block;margin:1.3rem 0 .35rem;text-transform:uppercase;font-size:.85rem;letter-spacing:.12em}
input[type=url],input[type=password],input[type=text],input:not([type]){width:100%;font:inherit;padding:.7rem .9rem;color:var(--ink);background:#f4fff0;border:4px solid var(--ink);border-radius:.9rem .3rem .9rem .3rem;box-shadow:5px 5px 0 var(--cyan);outline:0;transition:transform .12s,box-shadow .12s}
input:focus{transform:translate(-3px,-3px) rotate(-.4deg);box-shadow:9px 9px 0 var(--grape);background:#fff}
input[type=checkbox]{width:1.7rem;height:1.7rem;accent-color:var(--pink);vertical-align:middle;cursor:pointer}
button{font:inherit;text-transform:uppercase;letter-spacing:.1em;cursor:pointer;margin-top:1.6rem;padding:.85rem 1.6rem;color:var(--ink);background:var(--lime);border:4px solid var(--ink);border-radius:2rem;box-shadow:6px 6px 0 var(--ink);transition:transform .1s,box-shadow .1s,background .2s}
button:hover{background:var(--pink);transform:translate(-2px,-2px) rotate(-2deg);box-shadow:9px 9px 0 var(--ink)}
button:active{transform:translate(5px,5px);box-shadow:1px 1px 0 var(--ink)}
.error{background:var(--pink);color:#fff;border:4px dashed var(--ink);border-radius:.8rem;padding:.7rem 1rem;transform:rotate(.8deg)}
table{border-collapse:separate;border-spacing:0 .6rem;width:100%}
th{text-align:left;font-size:.78rem;text-transform:uppercase;letter-spacing:.12em;padding:.2rem .7rem}
td{padding:.6rem .7rem;background:#fbf4ff;border-top:3px solid var(--ink);border-bottom:3px solid var(--ink)}
td:first-child{border-left:3px solid var(--ink);border-radius:1rem 0 0 1rem}td:last-child{border-right:3px solid var(--ink);border-radius:0 1rem 1rem 0}
tr:nth-child(even) td{background:#effcff}tbody tr:hover td{background:var(--sun)}
td input{box-shadow:3px 3px 0 var(--grape);padding:.45rem .6rem}
code,.ref{font-family:Consolas,monospace;font-size:.82rem;background:var(--ink);color:var(--lime);padding:.1rem .45rem;border-radius:.4rem}
nav{display:flex;flex-wrap:wrap;gap:.6rem;margin:0 0 1.2rem}
nav a{color:var(--ink);text-decoration:none;text-transform:uppercase;letter-spacing:.1em;font-size:.82rem;padding:.3rem .9rem;background:var(--cyan);border:3px solid var(--ink);border-radius:2rem;box-shadow:3px 3px 0 var(--ink);transition:transform .1s,background .2s}
nav a:nth-child(2){background:var(--lime)}nav a:nth-child(3){background:var(--cyan)}nav a:nth-child(4){background:var(--sun)}nav a:nth-child(5){background:var(--pink)}nav a:nth-child(6){background:var(--grape);color:#fff}
nav a:hover{transform:translate(-2px,-2px) rotate(-3deg);background:var(--sun)}
.course-tabs{border-bottom:4px solid var(--ink);padding:.6rem .2rem 1rem;margin-top:1.2rem}
nav.course-tabs a{background:#fbf4ff;color:var(--ink)}
.course-tabs a[aria-current=page]{background:var(--lime);color:var(--ink);box-shadow:5px 5px 0 var(--grape);transform:translateY(-3px)}
.table-scroll{overflow-x:auto;padding:0 .3rem .5rem}
table.students{min-width:48rem}table.students input:not([type]){min-width:10rem}
.student-name{min-width:10rem}.student-name .ref{display:inline-block;margin-top:.25rem;font-size:.7rem}
.diff-tag{background:var(--cyan);margin:.15rem .2rem .15rem 0;text-transform:none}
.diff-tag.silver{background:#d7dce5}.diff-tag.red{background:var(--pink);color:#fff}.diff-tag.blue{background:var(--cyan)}
.ok{background:var(--lime);border:4px solid var(--ink);border-radius:.8rem;padding:.7rem 1rem;transform:rotate(-.6deg)}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(15rem,1fr));gap:1.4rem;margin:1.2rem 0}
.card{border:4px solid var(--ink);border-radius:1.4rem .5rem 1.4rem .5rem;padding:1rem 1.2rem;background:#fbf4ff;box-shadow:6px 6px 0 var(--grape)}
.card:nth-child(2n){background:#effcff;box-shadow:6px 6px 0 var(--pink);transform:rotate(.6deg)}
.card:nth-child(2n+1){transform:rotate(-.6deg)}
.card h2{margin:0 0 .4rem;font-size:1.1rem;text-transform:uppercase;letter-spacing:.08em}
.card code{overflow-wrap:anywhere}
.card a.go{display:inline-block;margin-top:.6rem;color:var(--ink)}
.pill{display:inline-block;padding:.05rem .7rem;border:3px solid var(--ink);border-radius:2rem;font-size:.8rem;text-transform:uppercase}
.pill.on{background:var(--lime)}.pill.off{background:var(--pink);color:#fff}
.inline{display:flex;gap:.6rem;flex-wrap:wrap;align-items:center}.inline input{flex:1 1 14rem}.inline button{margin-top:0}
.row{display:flex;gap:.8rem;flex-wrap:wrap;align-items:center;margin:1rem 0}.row form{margin:0}.row button{margin-top:0}
.reveal{width:1.5rem;height:1.5rem;accent-color:var(--pink);vertical-align:middle}
label.pill{cursor:pointer;background:var(--cyan)}
select{font:inherit;padding:.5rem;border:4px solid var(--ink);border-radius:.8rem;background:#f4fff0}
.deck{display:grid;grid-template-columns:repeat(auto-fill,minmax(14rem,1fr));gap:1.8rem 1.6rem;margin:1.4rem 0 .6rem;perspective:1200px}
h2.grp{margin:1.6rem 0 0;text-transform:uppercase;letter-spacing:.1em;font-size:1.1rem}
.t-normal{--c1:#a8a77a;--c2:#d6d5b0}.t-fire{--c1:#ee8130;--c2:#f5ac78}.t-water{--c1:#6390f0;--c2:#9db7f5}.t-electric{--c1:#f7d02c;--c2:#fae078}.t-grass{--c1:#7ac74c;--c2:#a7db8d}.t-ice{--c1:#96d9d6;--c2:#bce6e6}.t-fighting{--c1:#c22e28;--c2:#d67873}.t-poison{--c1:#a33ea1;--c2:#c183c1}.t-ground{--c1:#e2bf65;--c2:#ebd69d}.t-flying{--c1:#a98ff3;--c2:#c6b7f5}.t-psychic{--c1:#f95587;--c2:#fa92b2}.t-bug{--c1:#a6b91a;--c2:#c6d16e}.t-rock{--c1:#b6a136;--c2:#d1c17d}.t-ghost{--c1:#735797;--c2:#a292bc}.t-dragon{--c1:#6f35fc;--c2:#a27dfa}.t-dark{--c1:#705746;--c2:#a29288}.t-steel{--c1:#b7b7ce;--c2:#d1d1e0}.t-fairy{--c1:#d685ad;--c2:#f4bdc9}
.pk{position:relative;overflow:hidden;border:4px solid var(--ink);border-radius:1.3rem .6rem 1.3rem .6rem;padding:.8rem .9rem 1rem;background:linear-gradient(160deg,#fff 0,var(--c2) 48%,var(--c1) 100%);box-shadow:6px 6px 0 var(--ink);animation:deal .75s cubic-bezier(.2,1.4,.4,1) both;animation-delay:calc(var(--i)*70ms);transition:transform .2s}
@keyframes deal{from{opacity:0;transform:translateY(46px) rotateY(-100deg) scale(.7)}to{opacity:1;transform:none}}
.pk:hover{transform:translateY(-7px) rotate(-1.4deg)}
.pk::after{content:"";position:absolute;inset:0;pointer-events:none;background:linear-gradient(115deg,transparent 30%,rgba(255,255,255,.75) 45%,rgba(255,170,255,.45) 50%,transparent 62%);transform:translateX(-130%)}
.pk:hover::after{transform:translateX(130%);transition:transform .9s}
.pk .top{display:flex;justify-content:space-between;align-items:center;font-size:.78rem;text-transform:uppercase;letter-spacing:.08em}
.pk .tag{background:var(--ink);color:#fff;padding:.05rem .6rem;border-radius:2rem}
.pk .art{width:8.5rem;height:8.5rem;margin:.4rem auto 0;animation:float 3.2s ease-in-out infinite;animation-delay:calc(var(--i)*-.4s);filter:drop-shadow(0 7px 0 rgba(27,11,58,.28))}
@keyframes float{0%,100%{transform:translateY(0) rotate(-2.5deg)}50%{transform:translateY(-9px) rotate(2.5deg)}}
.art svg{width:100%;height:100%;overflow:visible}
.art .body{fill:var(--c1);stroke:var(--ink);stroke-width:3}.art .belly{fill:#fff;opacity:.55}
.art .part{fill:var(--c1);stroke:var(--ink);stroke-width:3;filter:brightness(.9)}
.art .stalk{fill:none;stroke:var(--ink);stroke-width:5;stroke-linecap:round}.art .tip{fill:var(--c2);stroke:var(--ink);stroke-width:3}
.art .bolt{fill:none;stroke:var(--ink);stroke-width:5;stroke-linejoin:round;stroke-linecap:round}
.art .cheek{fill:#ff7aa8;opacity:.85}.art .white{fill:#fff;stroke:var(--ink);stroke-width:2.5}.art .pupil{fill:var(--ink)}
.art .mouth{fill:none;stroke:var(--ink);stroke-width:3;stroke-linecap:round}.art .mouthfill{fill:var(--ink)}
.art .eyes{transform-box:fill-box;transform-origin:center;animation:blink 4.5s infinite}
@keyframes blink{0%,92%,100%{transform:scaleY(1)}95%{transform:scaleY(.08)}}
.pk h3{margin:.3rem 0 0;text-align:center;font-size:1.3rem;text-transform:uppercase;letter-spacing:.05em;-webkit-text-stroke:0}
.pk .real{margin:.1rem 0 0;text-align:center;font-size:.85rem;font-weight:700;transition:filter .25s}
#reveal:checked ~ section .deck .real{filter:blur(7px);user-select:none}
.pk .course{display:block;text-align:center;font-size:.72rem;text-transform:uppercase;letter-spacing:.1em;opacity:.8}
.pk .acts{display:flex;gap:.4rem;justify-content:center;margin-top:.7rem;flex-wrap:wrap}.pk .acts form{margin:0}
.pk .acts button{margin:0;padding:.3rem .75rem;font-size:.7rem;border-width:3px;box-shadow:3px 3px 0 var(--ink)}
.pk.mon{outline:4px dashed var(--ink);outline-offset:3px}
.pk.shiny{border-color:var(--sun);box-shadow:6px 6px 0 var(--ink),0 0 0 4px var(--ink),0 0 22px 6px rgba(255,210,63,.9)}
.pk.shiny .top .tag::before{content:"º6 shiny º6";animation:sparkle 1.2s ease-in-out infinite}
@keyframes sparkle{0%,100%{opacity:1}50%{opacity:.35}}
.ball{position:absolute;inset:0;z-index:3;display:grid;place-items:center;pointer-events:none;background:var(--sun);animation:popball 1.7s ease-in both;animation-delay:calc(var(--i)*70ms + .45s)}
.ball i{display:block;width:4.6rem;height:4.6rem;border-radius:50%;border:5px solid var(--ink);background:linear-gradient(#ff3e3e 0 44%,var(--ink) 44% 56%,#fff 56%);animation:ballshake .45s ease-in-out 3 both;animation-delay:calc(var(--i)*70ms + .45s)}
@keyframes ballshake{0%,100%{transform:rotate(0)}25%{transform:rotate(-22deg)}75%{transform:rotate(22deg)}}
@keyframes popball{0%,72%{opacity:1;transform:none}100%{opacity:0;visibility:hidden;transform:scale(1.6)}}
.swatch{display:inline-block;width:1rem;height:1rem;border:3px solid var(--ink);border-radius:50%;vertical-align:middle}
table.courses td input[type=text]{min-width:10rem}
textarea{width:100%;font:inherit;font-weight:400;padding:.7rem .9rem;color:var(--ink);background:#f4fff0;border:4px solid var(--ink);border-radius:.9rem .3rem .9rem .3rem;box-shadow:5px 5px 0 var(--cyan);outline:0;resize:vertical}
textarea:focus{box-shadow:9px 9px 0 var(--grape);background:#fff}
td textarea{box-shadow:3px 3px 0 var(--grape);padding:.45rem .6rem;min-width:16rem;font-size:.9rem}
.card.voice{transform:none;margin-top:1.6rem}
button.small{margin-top:.6rem;padding:.4rem .9rem;font-size:.75rem;box-shadow:4px 4px 0 var(--ink)}
button.danger{background:var(--pink);color:#fff}
details.forget{margin-top:.8rem;font-size:.85rem}details.forget summary{cursor:pointer;text-decoration:underline}
label.check{text-transform:none;letter-spacing:0;font-size:.85rem;margin:.6rem 0 0}
[popover]{max-width:32rem;width:calc(100% - 2rem);margin:auto;padding:1.2rem 1.4rem;color:var(--ink);background:#fff;border:5px solid var(--ink);border-radius:1.4rem .5rem 1.4rem .5rem;box-shadow:8px 8px 0 var(--pink);font:inherit}
[popover]::backdrop{background:rgba(27,11,58,.55)}
[popover] h2{margin:0 0 .6rem;font-size:1.2rem;text-transform:uppercase;letter-spacing:.06em}[popover] ol{margin:.4rem 0 .6rem;padding-left:1.4rem}[popover] li{margin:.35rem 0}
.row button[popovertarget]{margin-top:0}
.pill.same{background:var(--lime)}.pill.differs{background:var(--sun)}
ul.playbook{list-style:none;margin:.6rem 0 0;padding:0}ul.playbook li{margin:.7rem 0;font-size:.9rem}ul.playbook code{overflow-wrap:anywhere}
@media (prefers-reduced-motion:reduce){.ball{display:none}}
.done{text-align:center}.done h1{font-size:3rem}
@media (prefers-reduced-motion:reduce){*{animation:none!important;transition:none!important}}
@media (max-width:40rem){main{padding:1.4rem 1.1rem}h1{font-size:1.8rem}}
`
