// A scripted provider for tests: each call returns the next reply, and the
// requests it saw are kept for assertions.

/**
 * @param {Array<object | ((req: object) => object)>} replies
 * @param {{ supportsTools?: boolean }} opts
 */
export function fakeProvider(replies, { supportsTools = true } = {}) {
  const requests = [];
  return {
    name: 'fake',
    model: 'fake',
    supportsTools,
    requests,
    async complete(req) {
      requests.push(structuredClone({ ...req, signal: undefined }));
      const next = replies.shift();
      if (!next) return { text: 'Done.', toolCalls: [], stop: 'end', raw: {} };
      const reply = typeof next === 'function' ? await next(req) : next;
      return {
        text: '',
        toolCalls: [],
        stop: reply.toolCalls?.length ? 'tool_use' : 'end',
        raw: {},
        ...reply,
      };
    },
  };
}
