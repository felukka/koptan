import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { test } from 'node:test';
import { createTools } from '../src/agent/tools/index.js';
import { tempDir } from './helpers.js';

async function workspace() {
  const root = await tempDir();
  const outside = await tempDir();
  await fs.writeFile(path.join(outside, 'secret'), 's3cret');
  await fs.mkdir(path.join(root, 'src'));
  await fs.mkdir(path.join(root, '.git'));
  await fs.writeFile(path.join(root, 'src', 'app.js'), 'const a = 1;\nconst b = 1;\n');
  await fs.symlink(path.join(outside, 'secret'), path.join(root, 'leak'));
  await fs.symlink(outside, path.join(root, 'leakdir'));
  return { root, outside };
}

test('file tools stay inside the workspace', async () => {
  const { root, outside } = await workspace();
  const tools = createTools({ root, allowCommands: false });
  for (const p of ['../x', '/etc/passwd', 'leak', 'leakdir/secret', '.git/config', 'src/../../x']) {
    const read = await tools.run('read_file', { path: p });
    assert.equal(read.isError, true, `read ${p} must fail`);
    assert.doesNotMatch(read.output, /s3cret/);
  }
  const write = await tools.run('write_file', {
    path: 'leakdir/new.txt',
    content: 'x',
  });
  assert.equal(write.isError, true);
  await assert.rejects(fs.access(path.join(outside, 'new.txt')));
});

test('write, edit, list, search and delete', async () => {
  const { root } = await workspace();
  const tools = createTools({ root, allowCommands: false });
  assert.equal(
    (
      await tools.run('write_file', {
        path: 'web/index.html',
        content: '<h1>hi</h1>',
      })
    ).isError,
    false,
  );
  const ambiguous = await tools.run('edit_file', {
    path: 'src/app.js',
    old_text: '= 1',
    new_text: '= 2',
  });
  assert.match(ambiguous.output, /appears 2 times/);
  assert.equal(
    (
      await tools.run('edit_file', {
        path: 'src/app.js',
        old_text: 'a = 1',
        new_text: 'a = $&2',
      })
    ).isError,
    false,
  );
  assert.equal(await fs.readFile(path.join(root, 'src/app.js'), 'utf8'), 'const a = $&2;\nconst b = 1;\n');
  const listing = (await tools.run('list_files', {})).output;
  assert.match(listing, /web\/index.html/);
  assert.doesNotMatch(listing, /\.git/);
  assert.match((await tools.run('search', { pattern: 'const b' })).output, /src\/app.js:2:/);
  assert.equal((await tools.run('delete_file', { path: 'web/index.html' })).isError, false);
});

test('inputs are checked and commands are gated', async () => {
  const { root } = await workspace();
  const off = createTools({ root, allowCommands: false });
  assert.ok(!off.definitions.some((d) => d.name === 'run_command'));
  assert.match((await off.run('run_command', { command: 'id' })).output, /unknown tool/);
  assert.match((await off.run('read_file', { path: 1 })).output, /must be string/);
  assert.match((await off.run('read_file', { path: 'a', extra: 1 })).output, /unknown field/);

  process.env.KOPTAN_GIT_TOKEN = 'must-not-leak';
  const on = createTools({ root, allowCommands: true });
  const out = await on.run('run_command', {
    command: 'echo "token=$KOPTAN_GIT_TOKEN"; pwd; exit 3',
  });
  assert.match(out.output, /^exit 3/);
  assert.match(out.output, new RegExp(root));
  assert.doesNotMatch(out.output, /must-not-leak/);
  delete process.env.KOPTAN_GIT_TOKEN;
});
