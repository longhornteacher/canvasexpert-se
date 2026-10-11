// Synthetic checks against the compiled Engine. No Canvas writes. The optional
// --review check uses the machine's saved connection only to start local review.
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, writeFile, readFile, copyFile } from 'node:fs/promises';
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
  assert.equal(names.length, 26);
  const configured = JSON.parse(await call('list_courses'));
  assert.deepEqual(configured, { source: 'configured', root, instance: origin, courses: [] });
  await call('list_courses', { source: 'unknown' }, true);
  assert.equal(await readFile(path.join(root, 'context/grading.md'), 'utf8'), 'Teacher-edited grading fixture.');
  assert.match(await call('read_workspace_file', { path: 'playbooks/setup.md' }), /source: canvas/);
  const coursesRead = await call('read_workspace_file', { path: 'context/courses.md' });
  const coursesHash = /sha256: ([a-f0-9]{64})/.exec(coursesRead)[1];
  await call('write_workspace_file', { path: 'context/courses.md', content: `---\nschema: 1\ninstance: ${origin}\naliases: true\ncourses:\n  - nickname: CS8\n    id: 11\n    current: true\n    timezone: America/Chicago\n  - nickname: OLD\n    id: 22\n    current: false\n---\nStarter file: no courses configured.\n`, expected_sha256: coursesHash });
  const updatedCourses = JSON.parse(await call('list_courses', { source: 'configured' }));
  assert.equal(updatedCourses.root, root);
  assert.deepEqual(updatedCourses.courses, [{ nickname: 'CS8', id: 11, current: true, timezone: 'America/Chicago' }, { nickname: 'OLD', id: 22, current: false }]);
  await call('engine_info');
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
  if (review) {
    const started = await call('start_review', { path: 'courses/assignment.md', course: 'CS8' }, false, true);
    assert.equal(started.state, 'pending');
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
  console.log(`PASS: 26 tools; bundled root from another directory, mismatch rejection, MCP-first seed preservation, fresh configured courses, compiled stdio, references, discovery, path exclusions, guarded context, binary asset and QuizForge/document validation${review ? ', real local review and revision status' : ''}. No Canvas writes.`);
  console.log(`Synthetic fixture: ${root}`);
} finally {
  child.stdin.end();
  const timer = setTimeout(() => child.kill(), 3000);
  await new Promise(resolve => { if (child.exitCode !== null) resolve(); else child.once('exit', resolve); });
  clearTimeout(timer);
}
