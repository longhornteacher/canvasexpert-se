package localui

import "fmt"

// SEExpansion keeps the CLI's edition label consistent with the WebUI.
func SEExpansion() string { return "Special Edition" }

// hubNavFor returns the shared primary and secondary navigation for a hub page.
// Its arguments are internal route labels, never request data.
func hubNavFor(primary, secondary string) string {
	primaryLink := func(label, href, key string) string {
		if primary == key {
			return fmt.Sprintf(`<a href="%s" aria-current="page">%s</a>`, href, label)
		}
		return fmt.Sprintf(`<a href="%s">%s</a>`, href, label)
	}
	var secondaryNav string
	items := map[string][][2]string{
		"Classroom": {{"Courses", "/courses"}, {"Students", "/students"}},
		"Settings":  {{"Connections", "/settings?section=connections"}, {"Preferences", "/settings?section=preferences"}, {"Files", "/settings?section=files"}, {"Agent", "/connect"}, {"System", "/settings?section=system"}},
		"Guide":     {{"How it works", "/how"}, {"Capabilities", "/capabilities"}},
	}
	if children := items[primary]; len(children) > 0 {
		secondaryNav = `<nav class="hub-subnav" aria-label="` + primary + `">`
		for _, item := range children {
			if item[0] == secondary {
				secondaryNav += fmt.Sprintf(`<a href="%s" aria-current="page">%s</a>`, item[1], item[0])
			} else {
				secondaryNav += fmt.Sprintf(`<a href="%s">%s</a>`, item[1], item[0])
			}
		}
		secondaryNav += `</nav>`
	}
	return `<header class="hub-header"><a class="hub-brand" href="/">CanvasExpert <span>SE</span></a><nav class="hub-nav" aria-label="Main navigation">` +
		primaryLink("Home", "/", "Home") + primaryLink("Classroom", "/courses", "Classroom") +
		primaryLink("Settings", "/settings", "Settings") + primaryLink("Guide", "/how", "Guide") +
		`</nav></header>` + secondaryNav
}

var hubNavHome = hubNavFor("Home", "")
var hubNavCourses = hubNavFor("Classroom", "Courses")
var hubNavStudents = hubNavFor("Classroom", "Students")
var hubNavConnections = hubNavFor("Settings", "Connections")
var hubNavPreferences = hubNavFor("Settings", "Preferences")
var hubNavFiles = hubNavFor("Settings", "Files")
var hubNavAgent = hubNavFor("Settings", "Agent")
var hubNavSystem = hubNavFor("Settings", "System")
var hubNavHow = hubNavFor("Guide", "How it works")
var hubNavCapabilities = hubNavFor("Guide", "Capabilities")

// funkyCSS is the shared look for the loopback pages. The name is retained to
// avoid a broad template migration while the pages move to the Bench shell.
const funkyCSS = `
:root{color-scheme:light dark;--page:#f3f4f2;--surface:#fff;--surface-soft:#f8f9f7;--ink:#242824;--muted:#646b65;--line:#d8ddd8;--accent:#3e6252;--accent-soft:#e4ece7;--danger:#9a3434;--danger-soft:#f8e8e7;--good:#356347;--focus:#315e9a}
@media(prefers-color-scheme:dark){:root{--page:#171a18;--surface:#222724;--surface-soft:#282e2a;--ink:#e7ebe7;--muted:#afb8b0;--line:#414a43;--accent:#a5c5b2;--accent-soft:#2a3b31;--danger:#ffaaaa;--danger-soft:#452c2b;--good:#a4d4b1;--focus:#8ab7f4}}
*{box-sizing:border-box}html{background:var(--page)}body{margin:0;min-height:100vh;padding:1.25rem;color:var(--ink);font:400 15px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;background:var(--page)}
main{max-width:58rem;margin:0 auto;padding:1.4rem 1.6rem 2rem;background:var(--surface);border:1px solid var(--line);border-radius:.65rem;position:relative}main.wide{max-width:78rem}
.hub-header{display:flex;align-items:center;justify-content:space-between;gap:1rem;flex-wrap:wrap;padding-bottom:.8rem;border-bottom:1px solid var(--line);margin-bottom:.7rem}.hub-brand{font-weight:650;color:var(--ink);text-decoration:none;letter-spacing:-.02em}.hub-brand span{font-weight:450;color:var(--muted)}
.hub-nav,.hub-subnav{display:flex;align-items:center;gap:.2rem .35rem;flex-wrap:wrap;margin:0}.hub-nav a,.hub-subnav a{display:inline-block;padding:.38rem .65rem;border:1px solid transparent;border-radius:.35rem;color:var(--muted);font-size:.92rem;text-decoration:none}.hub-nav a:hover,.hub-subnav a:hover{background:var(--surface-soft);color:var(--ink)}.hub-nav a[aria-current=page],.hub-subnav a[aria-current=page]{background:var(--accent-soft);border-color:var(--line);color:var(--ink);font-weight:600}.hub-subnav{padding:.1rem 0 .75rem;border-bottom:1px solid var(--line);margin-bottom:1.25rem}.hub-nav a:focus-visible,.hub-subnav a:focus-visible,a:focus-visible,summary:focus-visible,button:focus-visible,input:focus-visible,select:focus-visible,textarea:focus-visible{outline:2px solid var(--focus);outline-offset:2px}
.home-status-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:.8rem;margin:.7rem 0 1.3rem}.home-status{min-width:0}.home-status h2{display:flex;align-items:center;gap:.5rem}.status-mark{display:inline-block;width:.65rem;height:.65rem;flex:none;border:1px solid var(--muted);border-radius:50%;background:transparent}.status-mark.good,.home-status[data-state="Connected"] [data-health-mark]{background:#4d8c5e;border-color:#4d8c5e}.status-mark.bad,.home-status[data-state="Token rejected"] [data-health-mark],.home-status[data-state="Unavailable"] [data-health-mark]{background:#a74646;border-color:#a74646}.status-mark.waiting,.home-status[data-state="Checking…"] [data-health-mark]{background:#b58332;border-color:#b58332}.health-state{font-weight:620;margin:.3rem 0}.health-note{color:var(--muted);overflow-wrap:anywhere}.home-status details{margin-top:.6rem;font-size:.9rem}.home-courses{margin:1rem 0}.section-head{display:flex;justify-content:space-between;align-items:center;gap:.75rem;border-bottom:1px solid var(--line);padding-bottom:.35rem}.section-head h2{margin:0}.count{font-weight:450;color:var(--muted);font-size:.9em}.home-course-list{list-style:none;padding:0;margin:.25rem 0}.home-course-list li{display:flex;justify-content:space-between;gap:.75rem;padding:.48rem .1rem;border-bottom:1px solid var(--line)}.home-course-list li span{color:var(--muted);font-size:.88rem}.home-agent{display:flex;align-items:center;gap:.7rem;flex-wrap:wrap;border-top:1px solid var(--line);padding-top:.7rem;margin-top:1rem}.home-agent span{color:var(--muted)}
h1{margin:.2rem 0 .65rem;font-size:1.65rem;line-height:1.2;font-weight:620;letter-spacing:-.025em}h2{font-size:1.1rem;line-height:1.3;font-weight:620;margin:0 0 .45rem}h3{font-size:1rem;font-weight:620}p{margin:.5rem 0;color:var(--ink)}p.lead{font-size:1rem;max-width:54rem}small,.course-state{color:var(--muted)}
a{color:var(--accent);text-underline-offset:.15em}.go{display:inline-block;margin-top:.55rem;font-weight:550}.badge{display:none}
label{display:block;margin:.9rem 0 .3rem;font-size:.9rem;font-weight:550}input[type=url],input[type=password],input[type=text],input:not([type]),textarea,select{max-width:100%;font:inherit;font-weight:400;color:var(--ink);background:var(--surface);border:1px solid #aeb7af;border-radius:.35rem;padding:.5rem .6rem}input[type=url],input[type=password],input[type=text],input:not([type]),select{min-height:2.4rem}textarea{width:100%;resize:vertical}input:focus,select:focus,textarea:focus{border-color:var(--focus)}input[type=checkbox]{width:1rem;height:1rem;accent-color:var(--accent);vertical-align:middle;cursor:pointer}label.check{font-size:.9rem;font-weight:400;margin:.5rem 0}
button{font:inherit;font-weight:550;cursor:pointer;margin-top:.8rem;padding:.48rem .8rem;color:var(--ink);background:var(--surface);border:1px solid #aeb7af;border-radius:.35rem;transition:background .12s,border-color .12s}button:hover{background:var(--surface-soft);border-color:var(--muted)}button:active{background:var(--accent-soft)}button:disabled{cursor:not-allowed;opacity:.55}button.small{margin-top:.5rem;padding:.35rem .65rem;font-size:.9rem}button.danger{color:var(--danger);border-color:var(--danger)}
.error{color:var(--danger);background:var(--danger-soft);border:1px solid var(--line);border-radius:.35rem;padding:.55rem .7rem;overflow-wrap:anywhere}.ok{color:var(--good);background:var(--accent-soft);border:1px solid var(--line);border-radius:.35rem;padding:.55rem .7rem;overflow-wrap:anywhere}.pill{display:inline-block;padding:.08rem .4rem;border:1px solid var(--line);border-radius:.3rem;font-size:.83rem;color:var(--muted);background:var(--surface-soft);text-transform:none}.pill.on,.pill.same{color:var(--good);background:var(--accent-soft)}.pill.off{color:var(--danger);background:var(--danger-soft)}.pill.warn,.pill.differs{color:var(--ink);background:var(--surface-soft)}
code,.ref{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:.88em;color:var(--ink);background:var(--surface-soft);padding:.08rem .28rem;border-radius:.2rem;overflow-wrap:anywhere;word-break:break-word}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,19rem),1fr));gap:.8rem;margin:1rem 0}.card{min-width:0;padding:.85rem 1rem;background:var(--surface);border:1px solid var(--line);border-radius:.45rem;box-shadow:none;transform:none}.card h2{margin-bottom:.45rem}.card code,.card .ref{overflow-wrap:anywhere}.card.voice{margin-top:.9rem}.card.token{transform:none}.card.t-green,.card.t-yellow,.card.t-red{border-color:var(--line);box-shadow:none}.card a.go{color:var(--accent)}.row,.inline{display:flex;gap:.55rem;flex-wrap:wrap;align-items:center;margin:.7rem 0}.row form{margin:0}.row button{margin-top:0}.inline input{flex:1 1 14rem;min-width:0}#sync_root{flex:1 1 20rem;min-width:0}.inline button{margin-top:0}.reassure{border-left:3px solid var(--line);padding:.5rem .75rem;color:var(--muted);max-width:58rem}.steps{padding-left:1.35rem}.steps li{margin:.3rem 0}
table{border-collapse:collapse;width:100%}th{text-align:left;font-size:.82rem;font-weight:600;color:var(--muted);padding:.45rem .55rem;border-bottom:1px solid var(--line)}td{padding:.45rem .55rem;border-bottom:1px solid var(--line);vertical-align:middle}tbody tr:hover td{background:var(--surface-soft)}td input{padding:.4rem .5rem}table.courses td input[type=text]{min-width:9rem}.table-scroll{overflow-x:auto;max-width:100%;padding-bottom:.25rem}table.students{min-width:48rem}table.students input:not([type]){min-width:9rem}.student-name{min-width:9rem}.student-name .ref{display:inline-block;margin-top:.2rem;font-size:.75rem}.diff-tag{margin:.1rem .15rem .1rem 0}.diff-tag.silver,.diff-tag.red,.diff-tag.blue{background:var(--surface-soft);color:var(--ink)}
nav.course-tabs{display:flex;gap:.2rem .4rem;flex-wrap:wrap;margin:.6rem 0;padding:.3rem 0;border-bottom:1px solid var(--line)}nav.course-tabs a{padding:.3rem .55rem;text-decoration:none;color:var(--muted);border-radius:.3rem}nav.course-tabs a[aria-current=page]{background:var(--accent-soft);color:var(--ink);font-weight:600}.course-state{font-size:.82em}.courses tr.unpublished{opacity:.7}.student-name .ref{background:none;padding:0;color:var(--muted)}
.settings-section{margin:.7rem 0 1.1rem;padding:.8rem 0;border-bottom:1px solid var(--line);min-width:0}.settings-section>h2{margin-bottom:.45rem}.settings-section.voice textarea{max-width:58rem}
.student-notes summary{cursor:pointer;color:var(--muted)}.student-notes textarea{width:100%;min-width:8rem;box-sizing:border-box}table.students td input:not([type]){min-width:7rem}
details{margin:.55rem 0}details summary{cursor:pointer;text-underline-offset:.15em}details.forget{margin-top:.75rem;font-size:.9rem}details.forget summary{text-decoration:underline}.playbook{list-style:none;margin:.6rem 0 0;padding:0}.playbook li{margin:.65rem 0;padding:.45rem 0;border-bottom:1px solid var(--line);font-size:.92rem}.playbook code{overflow-wrap:anywhere}
.deck{display:grid;grid-template-columns:repeat(auto-fill,minmax(14rem,1fr));gap:.8rem;margin:1rem 0}.pk{border:1px solid var(--line);border-radius:.45rem;padding:.8rem;background:var(--surface-soft);box-shadow:none;transform:none;animation:none}.pk:hover{transform:none}.pk .tag,.pk.mon,.pk.shiny{border-color:var(--line);box-shadow:none}.pk .art,.ball{display:none}.pk h3{text-align:left;font-size:1.05rem;text-transform:none}.pk .acts button{margin:0}.pk .course,.pk .top{letter-spacing:0;text-transform:none}
.cap-page{transform:none}.cap-legend{display:flex;gap:.5rem 1rem;flex-wrap:wrap;margin:1rem 0}.cap-status{display:inline-flex;align-items:center;gap:.4rem;color:var(--ink);background:var(--surface)}.cap-status .light{width:.65rem;height:.65rem;border:1px solid var(--line);border-radius:50%;background:var(--light);flex:none}.cap-status.green{--light:#4d8c5e}.cap-status.yellow{--light:#b58332}.cap-status.red{--light:#a74646}.cap-group{margin:1.5rem 0}.cap-group h2{font-size:1.15rem;margin:0 0 .35rem}.cap-list{list-style:none;padding:0;margin:.65rem 0}.cap-list>li+li{border-top:1px solid var(--line)}.cap-item{background:transparent;padding:.55rem .25rem}.cap-item summary{cursor:pointer;line-height:1.4}.cap-item summary:hover{text-decoration:underline}.cap-item[open]{background:var(--surface-soft)}.guide-topics{display:grid;gap:.5rem}.guide-topic>summary{cursor:pointer;font-weight:650}.guide-topic[open]>summary{margin-bottom:.65rem}.guide-topic ol{margin:.5rem 0;padding-left:1.4rem}.cap-state-mark{display:inline-grid;place-items:center;width:1rem;height:1rem;margin:0 .4rem 0 .1rem;color:var(--muted);font-size:.8rem;font-weight:650;vertical-align:middle}.cap-item p{border-top:1px solid var(--line);margin:.45rem 0 0;padding-top:.45rem;font-size:.9rem;font-weight:400;overflow-wrap:anywhere}.cap-next{margin-top:1.5rem}.cap-status-sr{position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0}
[popover]{max-width:34rem;width:calc(100% - 2rem);margin:auto;padding:1.1rem 1.3rem;color:var(--ink);background:var(--surface);border:1px solid var(--line);border-radius:.5rem;box-shadow:0 8px 30px #0003;font:inherit}[popover]::backdrop{background:#0006}[popover] h2{margin:0 0 .6rem}[popover] ol{margin:.4rem 0 .6rem;padding-left:1.4rem}[popover] li{margin:.35rem 0}.qmark{margin:0 0 0 .3rem;padding:0;width:1.6rem;height:1.6rem;line-height:1;vertical-align:middle}.done{text-align:center}.done h1{font-size:1.8rem}.course-state{color:var(--muted)}
@media(max-width:40rem){body{padding:.5rem}main,main.wide{padding:1rem .8rem;border-radius:.4rem}.hub-header{align-items:flex-start;flex-direction:column;gap:.45rem}.hub-nav{width:100%;gap:.1rem}.hub-nav a{padding:.35rem .5rem}.hub-subnav{gap:.1rem}.cards,.home-status-grid{grid-template-columns:1fr}table.courses{min-width:40rem}.table-scroll{overflow-x:auto}.home-course-list li{align-items:flex-start;flex-direction:column;gap:0}}
.draft-restored{display:flex;gap:.6rem;align-items:center;flex-wrap:wrap;border:1px solid var(--line);border-left:3px solid var(--accent);border-radius:.35rem;padding:.45rem .7rem;background:var(--surface-soft)}.draft-restored button{margin:0}
@media(prefers-reduced-motion:reduce){*{animation:none!important;transition:none!important}}
`
