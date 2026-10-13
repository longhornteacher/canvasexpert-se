> **Retired 2026-10-13.** Implemented in Source (Engine, WebUI, playbooks, `TEACHER.md`) and verified by local fixture tests only. No real Antigravity, Claude or ChatGPT adoption has been observed; see `docs/engine-capabilities.md`.

# Handoff: Antigravity, Streamlined Auto-Install & Built-in Pedagogy

## Summary

This handoff details the design and implementation plan for three interconnected enhancements:
1. **Adding Google Antigravity as a first-class supported MCP client alongside Claude Desktop (Cowork) and ChatGPT (Work).**
2. **Re-architecting the teacher "install" workflow from an interrogative, single-client questionnaire into a proactive, friction-free auto-detection process that configures every available client on the machine in a single pass.**
3. **Establishing "Intent-Based Persona Routing" and "Built-in Pedagogy" (`TEACHER.md`) so that agents running in developer environments (Claude Code, Codex CLI, Antigravity) immediately pivot into expert classroom educators when interacting with teachers, adhering to concrete classroom design standards (100-point defaults, strong-student rigor, horizontal landscape layouts, writing exemplars, green quiz rationales).**

---

## Architectural Separation

Per [`AGENTS.md`](file:///d:/Development%20Projects/CanvasExpert-SE/AGENTS.md) and [`ARCHITECTURE.md`](file:///d:/Development%20Projects/CanvasExpert-SE/ARCHITECTURE.md):
* **Engine (Go code in `cmd/`, `internal/`)**: Owns client configuration parsing, safe file merging, backup preservation, process launching, and diagnostics. No instructional judgment.
* **Playbooks & Context (`workspace-template/`)**: Owns the teacher conversation, plain-language scripts, contextual guidance, course nickname proposals, pedagogical standards, and task kickoffs.
* **Shared Authority**: Credentials, courses (`context/courses.md`), and teacher preferences (`context/`) stay in **Install**. When multiple clients are registered, they all point to the same Install `--workspace`, sharing the single Canvas credential and course list with zero duplicate setup.

---

## Part 1: Antigravity MCP Integration

### Specification
* **Client identifier**: `"antigravity"`
* **Configuration path**: `~/.gemini/config/mcp_config.json` (resolved via `os.UserHomeDir()` or `%USERPROFILE%\.gemini\config\mcp_config.json`).
* **Transport**: Standard stdio transport using the exact same arguments as Work/Codex:
  ```json
  {
    "mcpServers": {
      "canvasexpert-se": {
        "command": "<InstallRoot>\\engine\\cse.exe",
        "args": [
          "--workspace",
          "<InstallRoot>",
          "mcp"
        ]
      }
    }
  }
  ```
* **Detection criterion**: Directory `~/.gemini/config` exists, or `~/.gemini/config/mcp_config.json` is present.
* **Safety & Preservation**:
  * If `mcp_config.json` does not exist, create it with parent directory `0700`.
  * If it exists, parse as JSON, preserve all other existing `mcpServers` entries, and update or insert `"canvasexpert-se"`.
  * Make a timestamped backup before writing (`mcp_config.json.cse-backup-<timestamp>`).
  * If the existing entry already targets the identical Install root, record `settings-already-saved` and leave unchanged.
  * If the existing entry targets a different Install root, require explicit replacement (`--replace-existing`), matching `registerWork`.

---

## Part 2: Proactive Multi-Client Auto-Detect Install Flow

### The Philosophy Shift
* **Old design**: Gated by an upfront question: *"Are you using Claude, ChatGPT, or both?"*, with the explicit constraint *"Never connect an app they did not choose."*
* **New design**: Proactive, silent inspection. The user is *already speaking to an agent inside an agentic app*, so at least one host is guaranteed. The Engine inspects the machine, connects all detected clients, and reports:
  > *"I found [X] on your computer and connected it! By the way, CanvasExpert can also work in [Y], so if you ever install that, just come right back here and I'll be happy to make sure it gets connected, too!"*

### Client Detection Inventory
| Client | Key | Detection Target | Config Action | User Action Required |
| :--- | :--- | :--- | :--- | :--- |
| **Claude Desktop** | `cowork` | `%APPDATA%\Claude` exists | Prepares `CanvasExpert-SE-Cowork-this-install.mcpb`, requests app launch or reveals file | Clicks "Approve" on one-time prompt |
| **ChatGPT / Codex** | `work` | `~/.codex` exists | Merges table into `~/.codex/config.toml` | Silent (restarts app if open) |
| **Antigravity** | `antigravity` | `~/.gemini` exists | Merges object into `~/.gemini/config/mcp_config.json` | Silent |

---

## Part 3: Streamlined Teacher Experience Touchpoints

### 1. Auto-Suggested Course Nicknames
Canvas course names are bloated bureaucratic strings (e.g. `2026-FALL-ENG-07-003-PREAP-SMITH-S1`). Rather than asking the teacher to type out 6 custom nicknames, the agent parses the names and proposes clean defaults:
> *"I found 4 active classes. Here are the short names I came up with:
> 1. **ELA 7 - Period 2** (`2026-FALL-ENG-07-002...`)
> 2. **Pre-AP ELA 7 - Period 3** (`2026-FALL-ENG-07-003...`)
> Do these look good, or would you like to tweak any?"*

### 2. Direct Canvas Token Page Link
Canvas access tokens are created at `https://<district>.instructure.com/profile/settings#access_tokens`.
* Once the teacher provides their district URL or opens the WebUI, offer a direct clickable link to their settings page.
* Provide two essential tips:
  * **Expiration**: *"Choose the end of the school year (or the maximum allowed like 90/180 days) so you don't have to redo this mid-semester."*
  * **Privacy**: *"This token is stored only in Windows Credential Manager on this machine. It never leaves your computer."*

### 3. The Local Safety Promise
Teachers fear AI accidentally changing student grades or publishing unfinished material. Give them a grounding 1-sentence reassurance:
> *"**Safety promise:** CanvasExpert drafts in private. It will never publish an assignment, alter a grade, or post feedback to students without showing you a private review page for your approval first."*

### 4. The "Split the Difference" Landing
Do not leave the user at a blank prompt, but do not overwhelm them with a 20-question intake form. Wrap up setup with three concrete, distinct doors:
> *"All set! CanvasExpert is connected to your courses, and everything is ready on this computer.
>
> To get our feet wet, what would help you most today?
> * 🎨 **Make a page look great:** Tell me a lesson topic, and whether you like your pages colorful or clean and simple.
> * 📝 **Build a quick quiz:** Tell me the topic and how many questions.
> * 📊 **Clear a grading stack:** Name an assignment with ungraded work, and we’ll review the submissions together.
>
> Pick whichever sounds easiest, or just tell me what's on your desk!"*

### 5. Just-in-Time Context & Visual Vibe
* Do not survey the teacher about CSS, pedagogical goals, or grading scales during install.
* Capture context **just-in-time** when a specific activity is triggered:
  * On first page build: ask about visual style (e.g., *Clean & Academic* vs. *Warm & Engaging*).
  * On first grading session: ask about rubric scale / feedback style.
* Automatically save these preferences to [`context/teaching-profile.md`](file:///d:/Development%20Projects/CanvasExpert-SE/workspace-template/context/teaching-profile.md) and [`context/grading.md`](file:///d:/Development%20Projects/CanvasExpert-SE/workspace-template/context/grading.md) so they persist across future sessions.

---

## Part 4: Intent-Based Persona Routing (`AGENTS.md`)

### The Problem
When teachers or dual teacher/developers open an Install workspace in developer tools (Claude Code, Codex CLI, Antigravity), the underlying model's system prompt defaults to software engineering ("write code, propose diffs, run terminal commands, inspect linters"). Without an explicit role pivot, the agent treats classroom tasks like coding bugs, talks in CLI syntax, and lacks pedagogical depth.

### The Solution: Top-of-File Fork
Place a routing check at the very top of `AGENTS.md` (both in the repository root and `workspace-template/AGENTS.md`):

```markdown
# CanvasExpert-SE Standing Instructions

## 1. Determine User Intent: Developer vs. Teacher

Before taking any action, determine how the user is approaching this session:

* **DEVELOPER MODE:** If the user is writing code, fixing bugs, compiling the Engine, 
  running tests, or discussing software architecture:
  -> Follow the **Source Development** guidelines below. Treat the user as a developer.

* **TEACHER MODE:** If the user asks about classroom tasks—such as creating an assignment, 
  designing a Canvas page, building a quiz, scoring student work, or configuring classes:
  -> **Immediately stop acting like a software engineer.**
  -> Read [TEACHER.md](TEACHER.md) and adopt the **Master Teacher Assistant** persona.
  -> Do not mention Go code, terminal commands, JSON schemas, or diffs. 
  -> Focus entirely on pedagogy, grade-level tone, learning targets, and the relevant playbook.
```

---

## Part 5: Built-in Pedagogy Standards (`TEACHER.md`)

A dedicated `TEACHER.md` document provides the pedagogical operating system for the agent when operating in Teacher Mode. It establishes non-negotiable classroom quality standards:

### 1. The 100-Point Rule (Clean Gradebook Math)
* **Standard:** All assignments and quizzes default to **100 points total**.
* **Rubrics:** Balance criteria evenly to total 100 (e.g., 4 criteria $\times$ 25 pts, or 5 criteria $\times$ 20 pts).
* **Quizzes:** Question points must be balanced or scaled to sum to 100 (e.g., 10 questions at 10 pts, or weighted 10/15 pts).
* **Rationale:** Parents, students, and Canvas gradebooks understand percentages intuitively. Arbitrary point totals (like 11 or 17 points) cause confusion when students miss 1–2 items.

### 2. Calibrated Rigor (Strong Students at Grade Level)
* **Standard:** Anchor text complexity, vocabulary, and question depth to **proficient-to-advanced students** at the requested grade level.
* **Guardrails:**
  * Avoid **infantilization** (treating 8th graders like 3rd graders with overly simplistic prompts).
  * Avoid **academic bloat** (generating college-level abstract jargon or convoluted passive voice).
  * Target Depth of Knowledge (DOK) 2 (application, context interpretation) and DOK 3 (strategic thinking, evaluating claims, justifying choices), with grade-appropriate Lexile vocabulary.
* **Quiz Distractor Science:** Wrong answers must represent *real, predictable student misconceptions*, never trick questions, jokes, obvious fakes, or "all of the above".

### 3. Screen Utilization (Fill Horizontal Space for Landscape Displays)
* **Standard:** Design for **11–14 inch Chromebooks and laptops in landscape**, not a 400px mobile phone screen.
* **Layout Rules:**
  * Never use narrow mobile-width containers (`max-width: 480px`) that leave massive empty white space on desktop monitors.
  * Use full-width headers or containers bounded gracefully to the viewport (`max-width: 100%` or `max-width: 1100px`).
  * Use responsive 2-column or 3-column card layouts (`display: flex; gap: 16px; flex-wrap: wrap;`) for daily agendas, vocabulary lists, and resource links.

### 4. Exemplars & Green Quiz Rationales
* **Writing Assignments:** Always include a **Grade-Representative Exemplar** (a high-scoring mentor text) in the directions or rubric overview showing students what an "A" looks like at their grade level.
* **Quiz Items (New Quizzes):** Every item must include a root `rationale` (or per-choice feedback) providing a **concise ~2-sentence explanation of why the correct answer is correct**.
  * **Visual Styling:** Format the explanation in green text so students immediately receive positive instructional reinforcement upon reviewing their submitted attempt:
    ```html
    <p style="color: #2e7d32; font-weight: 500;">
      <strong>Why this is correct:</strong> [2-sentence explanation of the concept and evidence]
    </p>
    ```

### 5. The "I Can" Target Anchor
* Every Canvas Page and Assignment opens with a prominent, single-sentence student-facing goal banner:
  > **🎯 Learning Target:** *I can [action verb] [concept/skill] so that [purpose/application].*

### 6. The 3-Step Task Checklist
* Format student instructions as an actionable, scannable 3-step checklist rather than narrative paragraphs:
  * **Step 1: Read & Examine** (Link to text, prompt, or video)
  * **Step 2: Plan & Draft** (Expectations, length, and criteria)
  * **Step 3: Review Rubric & Submit** (Self-check before submission)

### 7. Hub-Style Tiered Differentiation (Proactive Scaffolding)
* **The Proactive Offer:** Whenever a teacher asks to author a substantial assignment, project, or quiz, the agent proactively offers CE-SE's signature Hub-style differentiation:
  > *"Would you like to tier this using our Hub style? We can keep one assignment for the whole class, but add private tiered support pages (like sentence stems and graphic organizers for students who need more support, and an extension challenge for advanced kids)."*
* **The "Hub" Architecture:**
  * Rather than creating multiple separate assignments (which fractures the gradebook, creates administrative clutter, and tracks students), CE-SE maintains **one whole-class assignment** containing verified links to restricted support pages using `{{tier:LABEL}}`.
  * In Canvas, each support page is restricted to students carrying that specific **differentiation tag** (or section/group). Students only see their designated support page (other links yield Canvas's access-denied page).
* **Tiered Scaffolding Menu:**
  * **High-Support / Emerging Tier:** Sentence starters/frames, visual graphic organizers, vocabulary word banks, and chunked step-by-step thinking guides.
  * **On-Level / Target Tier:** Guiding questions, structural outlines, and self-monitoring checklists.
  * **Advanced / Extension Tier:** DOK 3/4 extension tasks, counter-perspective prompts, and creative synthesis constraints.
* **The Golden Pedagogical Rule:** *Tier supports scaffold the same standard; they never lower it.* All students are held to the same 100-point learning target; the support pages simply provide calibrated bridges to reach it.
* **Tag Respect:** The agent asks for the teacher's exact Canvas differentiation tags or checks `context/`—it never assumes or hardcodes placeholder tags.

---

## Implementation Plan

### Step 1: Engine Updates (`internal/installation/`)
1. **Extend `Availability` struct**:
   * Add `AntigravityConfigFound bool` and `AntigravityConfigPath string`.
   * Update `Inspect(root)` to probe `~/.gemini/config/`.
2. **Implement `registerAntigravity(root, exe, path string, replace bool) (Result, error)`**:
   * Reads and parses `~/.gemini/config/mcp_config.json`.
   * Follows the exact safety and backup pattern as `registerWork`.
3. **Add Multi-Client Registration**:
   * Implement `RegisterAuto(root string, replace bool) ([]Result, error)` to register all detected clients in one call.
   * Update `Register(root, client, replace)` to accept `"antigravity"`, `"work"`, `"cowork"`, and `"auto"` / `"all"`.
4. **Update `internal/installlog/log.go`**:
   * Add `"antigravity"` to the client allowlist.
5. **Unit Tests (`internal/installation/installation_test.go`)**:
   * Add tests for Antigravity JSON reading, backup creation, server entry insertion, and conflict handling.
   * Add test for multi-client auto-detection.

### Step 2: CLI Updates (`cmd/cse/main.go`)
1. **Update flag handling for `cse install`**:
   * Make `--client` optional (defaulting to `"auto"`).
   * Accept `--client antigravity`, `--client work`, `--client cowork`, or `--client all`.
   * When running in auto mode, register all detected clients, log each outcome, and output composite structured JSON.

### Step 3: WebUI Hub Updates (`internal/localui/`)
1. **Update `templates/connect.html`**:
   * Remove the rigid dropdown forcing a single choice between Claude and ChatGPT.
   * Display badges or status for all detected clients (Claude, ChatGPT, Antigravity) with their connection state.
2. **Update `connect.go`**:
   * Handle Antigravity and multi-client registration outcomes gracefully.

### Step 4: Playbook & Template Updates (`workspace-template/`)
1. **Create `workspace-template/TEACHER.md`**:
   * Implement the complete Pedagogical Operating System: 100-point defaults, strong-student rigor, horizontal landscape layouts, writing exemplars, green rationales, "I Can" banners, 3-step checklists, and proactive Hub-style tiered differentiation.
2. **Update `AGENTS.md` and `workspace-template/AGENTS.md`**:
   * Add Section 1: Intent-Based Persona Routing (Developer Mode vs. Teacher Mode).
   * Direct Teacher Mode immediately to `TEACHER.md`.
3. **`workspace-template/playbooks/install.md`**:
   * Rewrite Section 1 & 2: remove upfront interrogation; run `cse install` with auto-detection.
   * Add plain-language status reporting for connected apps.
   * Add the friendly non-blocking tip for undetected supported apps.
   * Add direct Canvas token settings URL link and token guidance.
   * Add automatic course nickname proposals.
   * Add the Local Safety Promise.
   * Add the "Split the Difference" 3-door kickoff menu.
4. **`workspace-template/playbooks/authoring.md` and `quizzes.md`**:
   * Incorporate the 100-point rule, landscape layout guidelines, writing exemplars, green rationales, and the proactive offer reflex for Hub-style tiered support pages.
5. **`workspace-template/context/teaching-profile.md`**:
   * Populate with sensible default pedagogy instead of blank skeleton strings (including standard scaffolding tiers: Emerging, On-Level, Extension).

### Step 5: Verification & Documentation
1. Run local test suite: `go test ./internal/installation/... ./internal/localui/... ./cmd/cse/...`.
2. Verify with mock configs for all three clients in a temporary fixture.
3. Update [`docs/engine-capabilities.md`](file:///d:/Development%20Projects/CanvasExpert-SE/docs/engine-capabilities.md) and [`internal/localui/capabilities.go`](file:///d:/Development%20Projects/CanvasExpert-SE/internal/localui/capabilities.go) to document Antigravity support, auto-client installation, and built-in pedagogical standards.
