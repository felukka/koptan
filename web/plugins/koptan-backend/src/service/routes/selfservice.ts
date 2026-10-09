import {
  ConflictError,
  InputError,
  NotFoundError,
  ServiceUnavailableError,
} from '@backstage/errors';
import {
  type CreateSelfServiceRequest,
  koptanSelfServiceCreatePermission,
  koptanSelfServicePromptPermission,
} from '@internal/plugin-koptan-common';
import type { Request, Response, Router } from 'express';
import { friendlyError } from '../k8s';
import { DNS_LABEL } from '../pipelines';
import {
  AgentClient,
  createSelfService,
  REFRESH_ANNOTATION,
  redactSelfService,
  validateCreateSelfService,
  watchDone,
} from '../selfservice';
import { authorize, type RouteContext, read } from './context';

const MAX_PROMPT = 8000;

/** :namespace/:name of a SelfService, validated. */
function target(req: Request) {
  const { namespace, name } = req.params;
  if (!DNS_LABEL.test(namespace) || !DNS_LABEL.test(name)) {
    throw new InputError('namespace and name must be DNS labels');
  }
  return { namespace, name };
}

/** Agent failures as Backstage errors: busy, missing or down. */
function agentFailure(e: unknown): unknown {
  const status = (e as { agentStatus?: number }).agentStatus;
  const message = (e as Error).message;
  if (status === 409)
    return new ConflictError('the agent is already working on a prompt');
  if (status === 404) return new NotFoundError(message);
  if (status === 503 || (e as { code?: string }).code === 'ECONNREFUSED') {
    return new ServiceUnavailableError('the agent is not running yet');
  }
  return e;
}

export function selfServiceRoutes(
  router: Router,
  ctx: RouteContext,
  agentKey?: string,
) {
  const agents = new AgentClient(ctx.client, agentKey);

  router.get('/selfservices', async (_req, res) => {
    const list = await read(() =>
      ctx.client.list('SelfService', ctx.namespace),
    );
    res.json(list.map(redactSelfService));
  });

  router.post('/selfservices', async (req, res) => {
    const credentials = await authorize(
      ctx,
      req,
      koptanSelfServiceCreatePermission,
    );
    const body = req.body as CreateSelfServiceRequest;
    const problem = validateCreateSelfService(body);
    if (problem) throw new InputError(problem);
    try {
      res
        .status(201)
        .json(
          await createSelfService(
            ctx.client,
            body,
            credentials.principal.userEntityRef,
          ),
        );
    } catch (e) {
      throw friendlyError(e, body);
    }
  });

  router.get('/selfservices/:namespace/:name/runs', async (req, res) => {
    const { namespace, name } = target(req);
    try {
      res.json(await agents.runs(namespace, name));
    } catch (e) {
      throw agentFailure(e);
    }
  });

  // Streams the agent's events to the browser. When the run pushed a
  // commit, the Service is annotated so the operator builds it at once.
  router.post(
    '/selfservices/:namespace/:name/runs',
    async (req: Request, res: Response) => {
      await authorize(ctx, req, koptanSelfServicePromptPermission);
      const { namespace, name } = target(req);
      const prompt = req.body?.prompt;
      if (
        typeof prompt !== 'string' ||
        !prompt.trim() ||
        prompt.length > MAX_PROMPT
      ) {
        throw new InputError(`prompt must be 1-${MAX_PROMPT} characters`);
      }
      const abort = new AbortController();
      const stream = await agents
        .start(namespace, name, prompt, abort.signal)
        .catch((e) => {
          throw agentFailure(e);
        });
      res.writeHead(200, {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache, no-transform',
        'X-Accel-Buffering': 'no',
      });
      res.flushHeaders();
      const refresh = (commit: string) =>
        ctx.client
          .annotate('Service', namespace, name, {
            [REFRESH_ANNOTATION]: commit,
          })
          .catch(() => undefined);
      res.on('close', () => abort.abort());
      stream.body.pipe(watchDone(refresh)).pipe(res);
    },
  );
}
