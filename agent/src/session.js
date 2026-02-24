// One run at a time: sync the repository, let the agent work, commit and
// push. Runs are kept in memory; git history is the durable record.
import { randomUUID } from 'node:crypto';
import fs from 'node:fs/promises';
import { runAgent } from './agent/loop.js';
import { commitMessage, systemPrompt } from './agent/prompt.js';
import { createTools } from './agent/tools/index.js';
import { log, mask } from './log.js';

const KEEP_RUNS = 50;
export const MAX_PROMPT = 8000;

export class BusyError extends Error {}

export class Session {
  /**
   * @param {{ repo: import('./git.js').Repo, provider: object, config: object }} deps
   */
  constructor({ repo, provider, config }) {
    Object.assign(this, { repo, provider, config });
    this.busy = false;
    this.runs = [];
  }

  /**
   * Runs one prompt. Events go to onEvent as they happen; the run record is
   * returned. Never throws for run failures, only for a busy session.
   */
  async run(prompt, onEvent = () => {}) {
    if (this.busy) throw new BusyError('a run is already in progress');
    this.busy = true;
    const run = {
      id: randomUUID(),
      prompt,
      status: 'running',
      startedAt: new Date().toISOString(),
    };
    this.runs.unshift(run);
    this.runs.length = Math.min(this.runs.length, KEEP_RUNS);
    const emit = (e) => {
      try {
        onEvent({ runId: run.id, ...e });
      } catch {
        // A client that went away must not stop the run.
      }
    };
    try {
      emit({ type: 'status', text: 'Syncing the repository' });
      await this.repo.prepare();
      const root = await fs.realpath(this.repo.dir);
      const tools = createTools({
        root,
        allowCommands: this.config.allowCommands,
      });
      const tree = (await tools.run('list_files', {})).output;
      const system = systemPrompt({ ...this.config, tree });
      emit({
        type: 'status',
        text: `Working with ${this.provider.name}/${this.provider.model}`,
      });
      const { summary } = await runAgent({
        provider: this.provider,
        tools,
        root,
        system,
        prompt,
        maxSteps: this.config.maxSteps,
        onEvent: emit,
      });
      emit({ type: 'status', text: 'Committing and pushing' });
      run.commit = await this.repo.commitAndPush(commitMessage(prompt, summary));
      run.summary = summary;
      run.status = 'succeeded';
      emit({ type: 'done', commit: run.commit, summary });
    } catch (e) {
      run.status = 'failed';
      run.error = mask(e.message);
      log('warn', 'run failed', { run: run.id, error: run.error });
      emit({ type: 'error', message: run.error });
    } finally {
      run.finishedAt = new Date().toISOString();
      this.busy = false;
    }
    return run;
  }
}
