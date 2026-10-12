# One lesson, multiple capability observations

Designed 2026-10-12 for teacher discussion. This is a proposed scenario, not authorization to select a live course, publish test content, submit as students or post assessment outputs. MCP validation remains deferred. Read Source AGENTS.md, ARCHITECTURE.md and SCORING-RULES.md before development; teacher execution reads Install AGENTS.md and the relevant playbooks first.

## Outcome and scope

Use one small Canvas-native lesson to obtain meaningful live evidence for authoring, assets, modules, ordinary scoring/discovery, New Quiz written work and student experience. Reuse the same objects and participants. The aim is successful teacher work, not a certification suite. A failed item does not block unrelated work or justify a new framework.

The core run can support ten promotions: quiz points; written-item authoring; Pages/rich directions; native modules; asset uploads; completion/resubmission discovery; lateness judgment; field rendering; New Quiz written-work scoring; student experience. Each promotion depends on its actual observations and supported limits, not merely completion of the scenario. Attachment reading, teaching quality and current CLI review also gain evidence; their broad labels may require narrower wording or further human/client observation. Sync/configuration are a separate optional add-on, and MCP stays yellow/deferred.

## Lesson and objects

Lesson: **Explain and repair a short algorithm**. The teacher selects a suitable subject/grade level; the algorithm is just a concrete starter, not a requirement.

Create only:

- One ordinary lesson Page: headings, emphasis, a small trace table, a short code block, a diagram with alt text, stable download links and a short captioned demonstration video if an MP4/VTT pair is readily available.
- One ordinary assignment: rich HTML directions, text/file submission as appropriate, two short response units, and an independent two-criterion rubric. Add two differentiated hub tiers at creation, each offering a useful scaffold for the same task. Use existing eligible non-collaborative differentiation tags; do not place restricted tier Pages in the ordinary module.
- One New Quiz: one objective item, two short essays and one file-upload item accepting DOCX/PDF. Set authored essay limits and file types. Give the quiz an independent rubric for comment-only scoring. Permit a second attempt if supported by the chosen task settings, but a New Quiz resubmission is not needed to prove ordinary assignment discovery.
- One native module ordered as SubHeader, lesson Page, downloadable File, ordinary Assignment and New Quiz Assignment.

Reuse one diagram, one student handout, one starter file and the optional MP4/VTT. Upload a harmless hidden-from-listing asset only if that access setting is useful to the lesson; do not expand into a permission matrix. No teacher solutions are uploaded. A small DOCX, text PDF and diagram image can also be student-created responses for the attachment-reading observation.

Use new exact IDs returned by the Engine and a unique `[CSE LESSON TEST]` prefix. Historical test-object IDs are not selected without checking current live state. The teacher selects the exact destination and whether publication is limited to a sandbox or a specifically scoped pilot. Prefer an existing isolated course with ordinary enrolled test accounts; keep gradebook/SIS impact out of real classroom results using supported settings and teacher verification.

## People and responsibilities

- **Coordinator:** designs the run, maintains scope and evidence, reviews focused fixes and decides which recorded claims support promotion.
- **Execution agent:** works from Install using the supported CLI for this Engine-focused run. Reads teacher instructions/playbooks, authors drafts, pulls work, uses its own reading tools, calculates proposed judgments, opens review and performs only authorized posts.
- **Teacher:** selects course, participants, publication/posting scope and accommodations; reviews authored content and assessments; enters New Quiz item points in SpeedGrader; judges voice and usefulness.
- **Three enrolled participants/accounts:** perform their own submissions and observe student access. Built-in Test Student may supplement ordinary assignment observations but cannot establish New Quiz or enrolled-student behavior. The teacher can operate permitted enrolled sandbox accounts; otherwise use willing participants through their normal access. Do not bypass identity/access controls or assume impersonation is available.
- **Implementation agent, only if needed:** fixes a demonstrated Engine failure in Source with focused tests. Playbook judgment corrections stay in the seed and teacher-approved active rules. Preserve drafts, receipts, attempts and unrelated changes.

Participants have deliberately small, complementary roles:

1. **A, tier one:** genuine ordinary submission, receives feedback, then submits a meaningful revision. The new attempt must reappear with earlier work. Compare with the strongest prior academic attempt, not the entered grade.
2. **B, tier two:** an explicitly identified placeholder followed by genuine work; the teacher chooses an actual deadline/calendar situation that makes the genuine timing meaningful. Their New Quiz upload is DOCX.
3. **C, no tier:** confirms restricted tier Pages are unavailable. A documented synthetic accommodation in the sandbox is already reflected in the Canvas effective deadline, so the agent must not add it twice. Their upload is a text PDF; a diagram response supplies the image-reading check.

Use honest live timestamps. Existing suitable sandbox attempt history can reduce waiting; otherwise submissions may span two sessions. Do not fabricate earlier submissions, timestamps or accommodations, or assume a date is a school holiday. Calendar coverage comes from the teacher's actual selected calendar. A calendar arithmetic example alone is not live evidence for attempt handling.

## Sequence

### 1. Prepare and author while unpublished

The teacher settles the small set of choices: destination, enrolled participants/tier mapping, synthetic work, deadline/accommodation setup, publication scope and posting scope. The agent checks current course configuration, enrolled identities, tags and active teacher rules. Teacher/student private pages and identity stores are not read as fixtures or evidence.

Upload assets with explicit access and retain receipts. Invoke an unchanged upload receipt again to exercise supported reuse rather than create a duplicate. Author the Page, assignment/hub and quiz, reviewing assignments/quizzes under the existing playbook gate. Assemble the module using returned IDs. Use normal supported resume execution of the same module document to establish that ordered entries are not duplicated; do not erase state or manufacture interrupted writes.

Before anyone submits the New Quiz, run `quiz-points` to change one named item's points. Observe the command's returned item points and total; inspect the quiz as part of the teacher's authoring review. The teacher resolves any intentional quiz-total mismatch in Canvas, because automatic total editing is not part of this command. Do not change points after submissions.

Pull the ordinary Page once for a real instructional revision, change one requested detail and push it. Read/inspect saved teaching HTML as part of content authoring, noting normalization and stable links. Do not introduce blanket post-write readback into grading.

### 2. Teacher preview and scoped publication

The teacher checks the lesson and assignment rendering, module order, video/captions when included, quiz Build/Preview, essay limits and accepted upload types. Distinguish a stored setting from an enforced student limit. A visible Canvas limitation is recorded plainly; only fix CE-SE if the Engine sent or represented something incorrectly.

Publish only the selected lesson objects under the teacher's chosen scope. Module publication does not publish children; use supported object publication or teacher Canvas steps where the Engine has no operation. Avoid a new generic publish command. Teacher-directed deadline/override setup stays in Canvas where Engine writes are unsupported.

### 3. Student experience and first attempts

Each participant opens the module and follows the lesson. Together observe the ordinary Page, image/download access, optional video/captions, quiz response/upload controls, and correct hub-tier access. A no-tier participant checks that a restricted Page cannot be opened through its direct link, not just that the link is absent. No student credentials or identifiable work enter Source.

Participants submit the ordinary assignment and New Quiz using the planned response formats. The teacher records whether limits visibly work. The Engine's first discovery/pull should find unfinished ordinary work; explicitly scope the New Quiz pull by its assignment ID, because broad pulls intentionally skip New Quizzes.

### 4. One scoring and review cycle

The agent opens downloaded DOCX/PDF/image work with its own tools. Preserve original bytes and report unavailable evidence honestly. No Engine conversion/extraction feature is added. Use the live rubric, exact attempts, teacher voice and calendar/student notes. For ordinary work, produce the computed assessment and honest criterion scores with any supported lateness timing. For New Quiz work, produce criterion scores and feedback only: no aggregate quiz assessment, grade, rubric assessment or lateness action.

Review ordinary multi-student results through the current CLI page: confirm the right attempt's text and attachment names appear beside exact proposed feedback. The teacher asks for one real feedback revision and checks the corrected review. This supplies useful current review evidence without claiming installed MCP behavior. Record the original loaded Engine build and client route.

After teacher review, post only the authorized results. Accepted writes count as done: report once and stop. Do not run confirmation reads or repeat posting just to prove duplicates are avoided. Student/teacher observations of feedback and SpeedGrader paragraphs are the requested usability activity, not an automated grading confirmation. The teacher enters the New Quiz per-item points separately after the comment-only output; this does not become evidence of an Engine grade write.

The teacher assesses whether scores and feedback are accurate, actionable and in their voice; participants assess whether they can find and understand the feedback. Resolve ordinary wording through playbooks/context rather than hardcoded Engine judgment.

### 5. Completion and genuine revision

As part of the next normal discovery step, completed ordinary attempts should be absent. A then submits the planned revision; discovery should expose the new attempt and pull its earlier history. Score/review/post that revision within the same authorized scope. Use the teacher's requested assessment, not a duplicate-write stress test. Preserve previous results and unsent work.

## Capability decisions after the run

| Yellow | Observation that supports green |
|---|---|
| Change New Quiz question points | Actual Engine write before submissions; requested item and totals shown correctly. Existing local submission guard remains accurately labeled. |
| What Canvas keeps/shows | Engine-authored Page/rich assignment/quiz rendering plus student feedback and teacher SpeedGrader paragraph display. Retain known field limits; do not imply every HTML construct is supported. |
| New Quiz written items | Authored essays/uploads, teacher Preview and actual student controls show the stored limits/types accurately, including any explicit Canvas limitation. Pulling is recorded under scoring. |
| Pages/rich directions | Engine create, pull/update and normal publication produce usable rendered instruction and working assets. |
| Native modules | Exact references/order, normal document reuse and student navigation work; no synchronization/deletion claim. |
| Uploads/receipts | Live uploads and receipt reuse work, student access matches requested settings; video/caption claim only if actually observed. Keep unobserved access modes explicit. |
| Completion/resubmission discovery | Ordinary enrolled attempts disappear after completion; a genuine new attempt appears with prior history. |
| Lateness judgment | Real timestamps, placeholder exclusion, known calendar and already-applied extension are handled correctly and teacher agrees with the basis. Canvas still calculates penalties. |
| New Quiz written-work scoring | Engine report/download path provides exact student/attempt evidence; reviewed comment-only outputs are accepted and usable. Teacher item scoring stays separate. |
| Student experience | Enrolled accounts complete the quiz and see feedback; both hub tiers and the no-tier restriction behave as intended. |
| Non-text attachment reading | Agent reads the actual supported DOCX/text-PDF/image responses. Narrow the label/evidence to demonstrated formats; scanned PDFs and arbitrary other formats are not implied. |
| Teaching quality/voice | Teacher finds the actual assessment and feedback useful and participants can understand it. Label the small sample honestly. |
| Local review | Current CLI shows work, revision and multi-student posting correctly. Do not promote the full mixed CLI/MCP entry while its installed-client gap is deferred without an agreed narrower label. |

The coordinator updates `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together only for claims supported by recorded outcomes. The color is scoped to the named capability, not perfection or coverage of all incidental cases. Keep remaining local-only and human/client evidence limits visible. A concise outcome record is sufficient; no certification metadata or new operation framework.

## Optional separate add-on: two-PC rules adoption

If two PCs and provider Sync are readily available, bind two Installs to a teacher-selected Sync directory using the already implemented workflow. Preserve active custom rules. Edit one harmless teacher preference on PC A, observe provider arrival on B and show that B's agent actually reads/adopts it; reverse one change. Confirm operational/student work stays local. This can support shared-rules promotion without Canvas work.

To close the separate Source/Install/Sync/configuration entry, first settle the existing extension-versus-Install update ownership, consolidate first-run guidance, and observe one fresh install/update retaining teacher files and intended roots. That is a bounded delivery task, not an incidental prerequisite for this lesson. Sync adoption alone cannot prove it. MCP end-to-end remains deferred.

## Evidence, repairs and finish

Keep teacher-run notes and private artifacts in Install, including exact object IDs, Engine build, route used, accepted outcomes, teacher/student observations, any failed item and its next action. Source receives only sanitized summaries and product changes. Do not copy credentials, signed URLs, teacher-only notes or student identities into handoffs. The teacher chooses whether to retain or delete the isolated lesson afterward; do not delete submissions or unsent work automatically.

If a demonstrated failure needs code, give an implementation agent the exact trigger, expected behavior, affected operation and one focused reproduction. Independent participants/objects continue. After a fix, rerun only the affected useful step and relevant local checks; do not restart the entire lesson or migrate folders. Deliver the observed capability moves, remaining specific gaps and any scoped follow-up.
