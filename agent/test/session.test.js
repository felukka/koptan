import assert from 'node:assert/strict';
import path from 'node:path';
import { test } from 'node:test';
import { Repo } from '../src/git.js';
import { fakeProvider } from '../src/providers/fake.js';
import { Session } from '../src/session.js';
import { author, bareRepo, gitIn, tempDir } from './helpers.js';

const config = {
  allowCommands: false,
  maxSteps: 10,
  branch: 'main',
  appPort: 8080,
  service: 'demo',
};

async function session(replies) {
  const remote = await bareRepo();
  const dir = path.join(await tempDir(), 'repo');
  const repo = new Repo({ url: remote, branch: 'main', dir, author });
  return {
    remote,
    repo,
    session: new Session({ repo, provider: fakeProvider(replies), config }),
  };
}

test('seeds an empty repository, then commits and pushes each run', async () => {
  const {
    remote,
    repo,
    session: s,
  } = await session([
    {
      toolCalls: [
        {
          id: '1',
          name: 'write_file',
          input: { path: 'server.js', content: 'ok' },
        },
      ],
    },
    { text: 'Add a server listening on $PORT' },
    { text: 'Nothing to change' },
  ]);
  await repo.prepare();
  const seed = await repo.seed('# demo\n');
  assert.equal(gitIn(remote, 'rev-parse', 'main'), seed);
  assert.equal(await repo.seed('# again\n'), null, 'seeding happens once');

  const events = [];
  const run = await s.run('add a server', (e) => events.push(e));
  assert.equal(run.status, 'succeeded', run.error);
  assert.equal(gitIn(remote, 'rev-parse', 'main'), run.commit);
  assert.equal(gitIn(remote, 'show', 'main:server.js'), 'ok');
  const message = gitIn(remote, 'log', '-1', '--format=%B', 'main');
  assert.match(message, /^Add a server listening on \$PORT/);
  assert.match(message, /Prompt:\nadd a server/);
  assert.deepEqual(events.at(-1), {
    runId: run.id,
    type: 'done',
    commit: run.commit,
    summary: run.summary,
  });

  const idle = await s.run('just look');
  assert.equal(idle.status, 'succeeded');
  assert.equal(idle.commit, null, 'no changes, no commit');
  assert.equal(s.runs.length, 2);
});

test('a failed run leaves nothing behind and the next run starts clean', async () => {
  const {
    remote,
    repo,
    session: s,
  } = await session([
    {
      toolCalls: [
        {
          id: '1',
          name: 'write_file',
          input: { path: 'half.txt', content: 'x' },
        },
      ],
    },
    { text: 'nope', stop: 'refusal' },
    { text: 'fine' },
  ]);
  await repo.prepare();
  await repo.seed('# demo\n');
  const failed = await s.run('do something');
  assert.equal(failed.status, 'failed');
  assert.match(failed.error, /declined/);
  const before = gitIn(remote, 'rev-parse', 'main');
  const next = await s.run('again');
  assert.equal(next.commit, null, 'the half-written file was discarded');
  assert.equal(gitIn(remote, 'rev-parse', 'main'), before);
});

test('one run at a time', async () => {
  let release;
  const gate = new Promise((r) => {
    release = r;
  });
  const { repo, session: s } = await session([
    async () => {
      await gate;
      return { text: 'done' };
    },
  ]);
  await repo.prepare();
  const first = s.run('slow');
  await new Promise((r) => setTimeout(r, 50));
  await assert.rejects(s.run('second'), /already in progress/);
  release();
  assert.equal((await first).status, 'succeeded');
});
