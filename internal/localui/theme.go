package localui

import (
	"html/template"
	"math/rand/v2"
)

// seExpansions: "SE" stands for something different every time.
var seExpansions = []string{
	"Special Edition", "Speed Edition", "Special Envelope", "Sneaky Eraser", "Sloth Express",
	"Seriously Excellent", "Sparkly Engine", "Spreadsheet Escape", "Snack Enabled", "Stapler Enthusiast",
	"Semi-Educational", "Suspiciously Efficient", "Sticker Emporium", "Silent Elf", "Shockingly Explainable",
}

// SEExpansion returns a random meaning of "SE".
func SEExpansion() string { return seExpansions[rand.IntN(len(seExpansions))] }

var pageFuncs = template.FuncMap{"se": SEExpansion}

// funkyCSS is the shared look for the loopback pages. Pure CSS (the pages run
// under a no-script content security policy), so every effect is animation or
// transform only and honours prefers-reduced-motion.
// hubNav is the navigation bar shown on every page when served by the hub.
const hubNav = `<nav><a href="/">Home</a><a href="/setup">Canvas</a><a href="/students">Students</a></nav>`

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
nav a:nth-child(2){background:var(--lime)}nav a:nth-child(3){background:var(--pink)}
nav a:hover{transform:translate(-2px,-2px) rotate(-3deg);background:var(--sun)}
.ok{background:var(--lime);border:4px solid var(--ink);border-radius:.8rem;padding:.7rem 1rem;transform:rotate(-.6deg)}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(15rem,1fr));gap:1.4rem;margin:1.2rem 0}
.card{border:4px solid var(--ink);border-radius:1.4rem .5rem 1.4rem .5rem;padding:1rem 1.2rem;background:#fbf4ff;box-shadow:6px 6px 0 var(--grape)}
.card:nth-child(2n){background:#effcff;box-shadow:6px 6px 0 var(--pink);transform:rotate(.6deg)}
.card:nth-child(2n+1){transform:rotate(-.6deg)}
.card h2{margin:0 0 .4rem;font-size:1.1rem;text-transform:uppercase;letter-spacing:.08em}
.card a.go{display:inline-block;margin-top:.6rem;color:var(--ink)}
.pill{display:inline-block;padding:.05rem .7rem;border:3px solid var(--ink);border-radius:2rem;font-size:.8rem;text-transform:uppercase}
.pill.on{background:var(--lime)}.pill.off{background:var(--pink);color:#fff}
.inline{display:flex;gap:.6rem;flex-wrap:wrap;align-items:center}.inline input{flex:1 1 14rem}.inline button{margin-top:0}
.done{text-align:center}.done h1{font-size:3rem}
@media (prefers-reduced-motion:reduce){*{animation:none!important;transition:none!important}}
@media (max-width:40rem){main{padding:1.4rem 1.1rem}h1{font-size:1.8rem}}
`
