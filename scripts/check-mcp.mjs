// Synthetic checks against the compiled Engine. No Canvas writes. The optional
// --review check uses the machine's saved connection only to start local review.
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, writeFile, readFile, copyFile, rename } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import readline from 'node:readline';
import assert from 'node:assert/strict';

const engine = path.resolve(process.argv[2] || 'dist/cse.exe');
const review = process.argv.includes('--review');
const origin = process.env.CSE_MCP_CHECK_ORIGIN || 'https://canvas.example.edu';
const root = await mkdtemp(path.join(tmpdir(), 'cse-mcp-check-'));
await mkdir(path.join(root, 'context'));
await mkdir(path.join(root, 'courses'));
await writeFile(path.join(root, 'context/courses.md'), `---\nschema: 1\ninstance: ${origin}\naliases: true\ncourses: []\n---\nSynthetic fixture.\n`);
// Exercise a bundled installation from an unrelated agent directory.
await mkdir(path.join(root, 'engine'));
await writeFile(path.join(root, 'context/grading.md'), 'Teacher-edited grading fixture.');
const installedEngine = path.join(root, 'engine/cse.exe');
await copyFile(engine, installedEngine);
const otherDirectory = await mkdtemp(path.join(tmpdir(), 'cse-agent-check-'));
const conflicting = spawnSync(installedEngine, ['--workspace', otherDirectory, 'mcp'], { windowsHide: true, cwd: otherDirectory, encoding: 'utf8' });
assert.equal(conflicting.status, 2);
assert.match(conflicting.stderr, /installation mismatch/);
const child = spawn(installedEngine, ['mcp'], { windowsHide: true, cwd: otherDirectory, stdio: ['pipe', 'pipe', 'pipe'] });
let sequence = 0;
const pending = new Map();
let diagnostics = '';
child.stderr.on('data', b => { diagnostics += b; });
const lines = readline.createInterface({ input: child.stdout });
lines.on('line', line => {
  const response = JSON.parse(line);
  const waiter = pending.get(response.id);
  if (waiter) { pending.delete(response.id); clearTimeout(waiter.timer); waiter.resolve(response); }
});
child.on('exit', code => { for (const waiter of pending.values()) { clearTimeout(waiter.timer); waiter.reject(new Error(`Engine exited ${code}: ${diagnostics}`)); } pending.clear(); });
function request(method, params) {
  const id = ++sequence;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => { pending.delete(id); reject(new Error(`${method} timed out`)); }, 45000);
    pending.set(id, { resolve, reject, timer });
    child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
  });
}
async function call(name, args = {}, expectError = false, structured = false) {
  const reply = await request('tools/call', { name, arguments: args });
  assert.equal(Boolean(reply.error || reply.result?.isError), expectError, JSON.stringify(reply));
  if (expectError) return;
  if (structured) return reply.result.structuredContent;
  return reply.result.structuredContent?.text || reply.result.content.map(c => c.text || '').join('\n');
}
try {
  const initialized = await request('initialize', { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'synthetic-check', version: '1' } });
  assert.equal(initialized.result.serverInfo.name, 'canvasexpert-se');
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');
  const discovered = await request('tools/list', {});
  const names = discovered.result.tools.map(t => t.name);
  assert.equal(names.length, 53);
  assert.ok(names.includes('init_results'), 'missing init_results');
  for (const name of ['inbox_list', 'inbox_read', 'inbox_reply', 'inbox_set_state']) assert.ok(names.includes(name), `missing ${name}`);
  const inboxReadTool = discovered.result.tools.find(t => t.name === 'inbox_read');
  assert.ok(inboxReadTool.inputSchema.properties.conversation, 'inbox_read must require a conversation ref');
  assert.ok(inboxReadTool.inputSchema.properties.mark_read, 'inbox_read must expose optional mark_read');
  const inboxReplyTool = discovered.result.tools.find(t => t.name === 'inbox_reply');
  for (const field of ['conversation', 'body', 'after_message']) assert.ok(inboxReplyTool.inputSchema.properties[field], `inbox_reply must expose ${field}`);
  const inboxStateTool = discovered.result.tools.find(t => t.name === 'inbox_set_state');
  for (const field of ['conversations', 'state']) assert.ok(inboxStateTool.inputSchema.properties[field], `inbox_set_state must expose ${field}`);
  const familyContactTool = discovered.result.tools.find(t => t.name === 'prepare_family_contact');
  assert.ok(familyContactTool?.inputSchema.properties.folder, 'prepare_family_contact must expose folder');
  const audienceTool = discovered.result.tools.find(t => t.name === "prepare_audience");
  assert.ok(audienceTool, "missing prepare_audience");
  for (const field of ["course", "path", "refresh"]) assert.ok(audienceTool.inputSchema.properties[field], `prepare_audience must expose ${field}`);
  assert.ok(names.includes('list_external_resources'), 'missing list_external_resources');
  const externalResourcesTool = discovered.result.tools.find(t => t.name === 'list_external_resources');
  assert.ok(externalResourcesTool.inputSchema.properties.course, 'list_external_resources must require course');
  assert.ok(externalResourcesTool.inputSchema.properties.group_id, 'list_external_resources must expose optional group_id');
  assert.ok(externalResourcesTool.inputSchema.properties.resource_type, 'list_external_resources must expose resource_type');
  await call('list_external_resources', { course: 'CS8', resource_type: 'invalid' }, true);
  for (const name of ['list_group_sets', 'list_groups', 'pull_group_set', 'pull_group_memberships']) assert.ok(names.includes(name), `missing ${name}`);
  assert.ok(names.includes('pull_assignment_dates'), 'missing pull_assignment_dates');
  const assignmentDatesTool = discovered.result.tools.find(t => t.name === 'pull_assignment_dates');
  assert.ok(assignmentDatesTool.inputSchema.properties.assignment_id, 'pull_assignment_dates must require exact assignment_id');
  for (const name of ['pull_page', 'list_page_revisions']) assert.ok(names.includes(name), `missing ${name}`);
  assert.ok(names.includes('pull_discussion'), 'missing pull_discussion');
  assert.ok(names.includes('pull_work'), 'missing pull_work');
  assert.ok(names.includes('prepare_late_timing'), 'missing prepare_late_timing');
  const prepareLateTool = discovered.result.tools.find(t => t.name === 'prepare_late_timing');
  for (const field of ['packet', 'students', 'groups']) assert.ok(prepareLateTool.inputSchema.properties[field], `prepare_late_timing must expose ${field}`);
  const pullWorkTool = discovered.result.tools.find(t => t.name === 'pull_work');
  assert.ok(pullWorkTool.inputSchema.properties.group_id, 'pull_work must expose optional group_id');
  assert.ok(pullWorkTool.inputSchema.properties.session, 'pull_work must expose scoring-session discovery');
  assert.match(pullWorkTool.description, /group-packet\/group-work/);
  await call('pull_work', { course: 'CS8', assignment: '10', group_id: 0 }, true);
  await call('pull_work', { course: 'CS8', group_id: 60 }, true);
  const discussionTool = discovered.result.tools.find(t => t.name === 'pull_discussion');
  assert.ok(discussionTool.inputSchema.properties.topic_id, 'pull_discussion must expose topic_id');
  assert.ok(discussionTool.inputSchema.properties.group_id, 'pull_discussion must expose optional group_id');
  assert.match(discussionTool.description, /discussion-packet/);
  assert.match(discussionTool.description, /pending/);
  const pageTool = discovered.result.tools.find(t => t.name === 'pull_page');
  assert.ok(pageTool.inputSchema.properties.query, 'pull_page must preserve the query input');
  assert.ok(pageTool.inputSchema.properties.group_id, 'pull_page must expose optional group_id');
  const configured = JSON.parse(await call('list_courses'));
  assert.equal(configured.source, 'configured');
  assert.equal(configured.root, root);
  assert.equal(configured.install_root, root);
  assert.equal(configured.rules_root, root);
  assert.equal(configured.mode, 'local');
  assert.equal(configured.instance, origin);
  assert.deepEqual(configured.courses, []);
  await call('list_courses', { source: 'unknown' }, true);
  await call('pull_page', { course: 'CS8', query: '7', group_id: -1 }, true);
  await call('pull_discussion', { course: 'CS8', topic_id: 7, group_id: -1 }, true);
  assert.equal(await readFile(path.join(root, 'context/grading.md'), 'utf8'), 'Teacher-edited grading fixture.');
  assert.match(await call('read_workspace_file', { path: 'playbooks/setup.md' }), /source: canvas/);
  const coursesRead = await call('read_workspace_file', { path: 'context/courses.md' });
  const coursesHash = /sha256: ([a-f0-9]{64})/.exec(coursesRead)[1];
  await call('write_workspace_file', { path: 'context/courses.md', content: `---\nschema: 1\ninstance: ${origin}\naliases: true\ncourses:\n  - nickname: CS8\n    id: 11\n    current: true\n    timezone: America/Chicago\n  - nickname: OLD\n    id: 22\n    current: false\n---\nStarter file: no courses configured.\n`, expected_sha256: coursesHash });
  const updatedCourses = JSON.parse(await call('list_courses', { source: 'configured' }));
  assert.equal(updatedCourses.root, root);
  assert.deepEqual(updatedCourses.courses, [{ nickname: 'CS8', id: 11, current: true, timezone: 'America/Chicago' }, { nickname: 'OLD', id: 22, current: false }]);
  await call('write_draft', { path: 'courses/mcp-late/packet.yaml', content: `---\nschema: 1\nkind: packet\ncourse: CS8\nassignment:\n  instance: ${origin}\n  course_id: 11\n  assignment_id: 20\n  title: MCP timing fixture\n  points: 10\n  submission_types: [online_text_entry]\nstudents: [courses/mcp-late/student.yaml]\n---\n` });
  await call('write_draft', { path: 'courses/mcp-late/student.yaml', content: `---\nschema: 1\nkind: student-work\nstudent_ref: s-mcp\nalias: Student\nassignment_id: 20\nassignment_visible: true\ncurrent:\n  ref:\n    attempt: 1\n    submitted_at: "2026-10-18T10:00:00-05:00"\n  effective_due_at: "2026-10-19T10:00:00-05:00"\n  observed_timing:\n    status_known: true\n    late_policy_status: null\n    late_known: true\n    late: false\n    seconds_late_known: true\n    seconds_late: 0\n    custom_status_known: true\n    has_custom_status: false\n---\n` });
  const preparedLate = JSON.parse(await call('prepare_late_timing', {
    packet: 'courses/mcp-late/packet.yaml',
    students: [{ student_ref: 's-mcp', first_genuine: { attempt: 1, submitted_at: '2026-10-18T10:00:00-05:00' } }],
  }));
  assert.equal(preparedLate.kind, 'late-timing-preparation');
  assert.equal(preparedLate.students[0].student_ref, 's-mcp');
  assert.equal(preparedLate.students[0].status, 'unchanged');
  await call('write_draft', { path: 'courses/mcp-late/decisions.yaml', content: `---\nschema: 1\nkind: late-timing-decisions\npacket: courses/mcp-late/packet.yaml\nstudents:\n  - student_ref: s-mcp\n    first_genuine:\n      attempt: 1\n      submitted_at: "2026-10-18T10:00:00-05:00"\n---\n` });
  const cliPreparedLate = spawnSync(installedEngine, ['--workspace', root, 'prepare-late-timing', 'courses/mcp-late/decisions.yaml'], { windowsHide: true, cwd: otherDirectory, encoding: 'utf8' });
  assert.equal(cliPreparedLate.status, 0, cliPreparedLate.stderr);
  assert.deepEqual(JSON.parse(cliPreparedLate.stdout), preparedLate, 'CLI and MCP preparation should agree without Canvas credentials');
  await call('engine_info');
  const syncRoot = await mkdtemp(path.join(tmpdir(), 'cse-rules-sync-'));
  const binding = JSON.parse(await call('engine_info', { operation: 'set-rules-location', sync_root: syncRoot }));
  assert.equal(binding.mode, 'sync');
  assert.equal(binding.install_root, root);
  assert.equal(binding.rules_root, syncRoot);
  const gradingRead = await call('read_workspace_file', { path: 'context/grading.md' });
  const gradingHash = /sha256: ([a-f0-9]{64})/.exec(gradingRead)[1];
  await call('write_workspace_file', { path: 'context/grading.md', content: 'Updated through shared authority.', expected_sha256: gradingHash });
  assert.match(await readFile(path.join(syncRoot, 'context/grading.md'), 'utf8'), /Updated through shared authority/);
  assert.match(await call('read_workspace_file', { path: 'context/grading.md' }), /Updated through shared authority/);
  const locationStatus = JSON.parse(await call('engine_info', { operation: 'rules-location' }));
  assert.equal(locationStatus.rules_root, syncRoot);
  assert.match(await call('engine_reference', { topic: 'formats' }), /rationales/);
  assert.match(await call('engine_reference', { topic: 'examples' }), /synthetic-quizforge-mixed/);
  await call('list_workspace_files');
  await call('write_draft', { path: 'courses/../context/escape.md', content: 'blocked' }, true);
  await call('read_workspace_file', { path: '.cse/students.csv' }, true);
  await call('write_workspace_file', { path: 'context/check.md', content: 'Synthetic context.' });
  const context = await call('read_workspace_file', { path: 'context/check.md' });
  const hash = /sha256: ([a-f0-9]{64})/.exec(context)[1];
  await call('write_workspace_file', { path: 'context/check.md', content: 'changed' }, true);
  await call('write_workspace_file', { path: 'context/check.md', content: 'Synthetic updated context.', expected_sha256: hash });
  const asset = Buffer.from('Synthetic binary fixture\0');
  await call('write_asset', { path: 'courses/asset.bin', base64: asset.toString('base64') });
  assert.equal(JSON.parse(await call('read_workspace_asset', { path: 'courses/asset.bin' })).base64, asset.toString('base64'));
  await call('write_draft', { path: 'courses/quiz.json', content: JSON.stringify({ title: 'Synthetic', items: [{ id: 'q1', type: 'ESSAY', prompt: 'Explain.', points: 5 }] }) });
  await call('validate_quiz', { path: 'courses/quiz.json' });
  const quizExample = await readFile('engine-docs/examples/synthetic-quizforge-mixed.md', 'utf8');
  await call('write_draft', { path: 'courses/mixed-quiz.txt', content: quizExample });
  await call('validate_quiz', { path: 'courses/mixed-quiz.txt' });
  await call('write_draft', { path: 'courses/invalid-settings.json', content: JSON.stringify({ title: 'Settings typo', metadata: { canvas: { allowed_atempts: 2 } }, items: [{ id: 'q1', type: 'ESSAY', prompt: 'Explain.', points: 5 }] }) });
  await call('validate_quiz', { path: 'courses/invalid-settings.json' }, true);
  const assignment = (await readFile('engine-docs/examples/synthetic-assignment-create.md', 'utf8')).replace('course: DEMO', 'course: CS8');
  await call('write_draft', { path: 'courses/assignment.md', content: assignment });
  await call('validate_document', { path: 'courses/assignment.md' });
  const groupAssignment = (await readFile('engine-docs/examples/synthetic-group-assignment.md', 'utf8')).replace('course: ELA7', 'course: CS8');
  await call('write_draft', { path: 'courses/group-assignment.md', content: groupAssignment });
  await call('validate_document', { path: 'courses/group-assignment.md' });
  const clubRefresh = await readFile('engine-docs/examples/synthetic-individual-club-refresh.md', 'utf8');
  await call('write_draft', { path: 'courses/individual-club-refresh.md', content: clubRefresh });
  await call('validate_document', { path: 'courses/individual-club-refresh.md' });
  const clubPage = await readFile('engine-docs/examples/synthetic-individual-club-page.md', 'utf8');
  await call('write_draft', { path: 'courses/individual-club-page.md', content: clubPage });
  await call('validate_document', { path: 'courses/individual-club-page.md' });
  const clubQuiz = await readFile('engine-docs/examples/synthetic-individual-club-quiz.md', 'utf8');
  await call('write_draft', { path: 'courses/individual-club-quiz.md', content: clubQuiz });
  await call('validate_quiz', { path: 'courses/individual-club-quiz.md' });
  // The integrated book-club unit example embeds one QuizForge block; validate it
  // and confirm the example still documents all three resource kinds.
  const unitExample = await readFile('engine-docs/examples/synthetic-book-club-unit.md', 'utf8');
  for (const kind of ['kind: page', 'kind: assignment', '<QUIZFORGE_JSON>', 'prepare-audience']) {
    assert.ok(unitExample.includes(kind), `book-club unit example must document ${kind}`);
  }
  const unitQuiz = unitExample.slice(unitExample.indexOf('<QUIZFORGE_JSON>'), unitExample.indexOf('</QUIZFORGE_JSON>') + '</QUIZFORGE_JSON>'.length);
  await call('write_draft', { path: 'courses/book-club-unit-quiz.txt', content: unitQuiz });
  await call('validate_quiz', { path: 'courses/book-club-unit-quiz.txt' });
  const clubAudience = await readFile('engine-docs/examples/synthetic-individual-club-assignment.md', 'utf8');
  await call('write_draft', { path: 'courses/individual-club-assignment.md', content: clubAudience });
  await call('validate_document', { path: 'courses/individual-club-assignment.md' });
  await call('prepare_audience', { course: '', path: 'courses/individual-club-assignment.md' }, true);
  const assignmentDates = await readFile('engine-docs/examples/synthetic-assignment-dates.md', 'utf8');
  await call('write_draft', { path: 'courses/assignment-dates.md', content: assignmentDates });
  await call('validate_document', { path: 'courses/assignment-dates.md' });
  const discussionPacket = await readFile('engine-docs/examples/synthetic-discussion-packet.yaml', 'utf8');
  await call('write_draft', { path: 'courses/discussion-packet.yaml', content: discussionPacket });
  await call('validate_document', { path: 'courses/discussion-packet.yaml' });
  const discussionReply = await readFile('engine-docs/examples/synthetic-discussion-reply.md', 'utf8');
  await call('write_draft', { path: 'courses/discussion-reply.md', content: discussionReply });
  await call('validate_document', { path: 'courses/discussion-reply.md' });
  if (review) {
    const started = await call('start_review', { path: 'courses/assignment.md', course: 'CS8' }, false, true);
    assert.equal(started.state, 'pending', JSON.stringify(started));
    const url = started.url;
    assert.match(url, /^http:\/\/127\.0\.0\.1:\d+\/review$/);
    const id = started.review_id;
    assert.ok(id);
    const html = await (await fetch(url)).text();
    const form = new URLSearchParams({ action: 'revise', status_0: 'revise', note_0: 'Synthetic revision: shorten directions.' });
    for (const match of html.matchAll(/name="(request_secret|token|ref_\d+)" value="([^"]*)"/g)) form.set(match[1], match[2]);
    const response = await fetch(new URL('/submit', url), { method: 'POST', headers: { Origin: new URL(url).origin }, body: form });
    assert.equal(response.status, 200, await response.text());
    let status;
    for (let i = 0; i < 20; i++) {
      status = await call('review_status', { review_id: id }, false, true);
      if (status.state !== 'pending') break;
      await new Promise(resolve => setTimeout(resolve, 50));
    }
    assert.equal(status.state, 'finished');
    assert.equal(status.outcome, 'held');
    assert.equal(status.result_path, path.join(root, 'courses/assignment.md'));
    assert.equal(status.url, undefined);
    assert.equal(status.revision_notes[0].note, 'Synthetic revision: shorten directions.');
    assert.match(status.text, /Outcome: held/);
    assert.doesNotMatch(status.text, /Open the local review page/);
    const state = await call('read_workspace_file', { path: 'courses/assignment.md.review.json' });
    assert.match(state, /Synthetic revision/);
  }
  await rename(syncRoot, `${syncRoot}-offline`);
  const unavailable = JSON.parse(await call('engine_info', { operation: 'rules-location' }));
  assert.equal(unavailable.mode, 'sync');
  assert.match(unavailable.availability_error, /unavailable/);
  await call('read_workspace_file', { path: 'context/grading.md' }, true);
  console.log(`PASS: ${names.length} tools; bundled root from another directory, mismatch rejection, MCP-first seed preservation, fresh configured courses, compiled stdio, group/Page/discussion/external-resource discovery, references, path exclusions, guarded context, binary asset and QuizForge/document validation${review ? ', real local review and revision status' : ''}. No Canvas writes.`);
  console.log(`Synthetic fixture: ${root}`);
} finally {
  child.stdin.end();
  const timer = setTimeout(() => child.kill(), 3000);
  await new Promise(resolve => { if (child.exitCode !== null) resolve(); else child.once('exit', resolve); });
  clearTimeout(timer);
}
