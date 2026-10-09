import { InputError } from '@backstage/errors';
import {
  type CreatePipelineRequest,
  koptanPipelineCreatePermission,
} from '@internal/plugin-koptan-common';
import type { Router } from 'express';
import { friendlyError } from '../k8s';
import {
  buildActivity,
  buildOverview,
  buildPipelines,
  createPipeline,
  loadAll,
  validateCreate,
} from '../pipelines';
import { authorize, type RouteContext, read } from './context';

/** Service -> CI -> CD views, cluster facts, and creating a Service. */
export function pipelineRoutes(router: Router, ctx: RouteContext) {
  const { client, namespace } = ctx;
  const all = () => read(() => loadAll(client, namespace));

  router.get('/overview', async (_req, res) => {
    const { services, cis, cds } = await all();
    res.json(buildOverview(services, cis, cds));
  });

  router.get('/pipelines', async (_req, res) => {
    const { services, cis, cds } = await all();
    res.json(buildPipelines(services, cis, cds));
  });

  router.get('/activity', async (_req, res) => {
    const { services, cis, cds } = await all();
    res.json(buildActivity(services, cis, cds));
  });

  router.get('/cluster', async (_req, res) => {
    res.json(await client.clusterInfo());
  });

  router.post('/pipelines', async (req, res) => {
    const credentials = await authorize(
      ctx,
      req,
      koptanPipelineCreatePermission,
    );
    const body = req.body as CreatePipelineRequest;
    const problem = validateCreate(body);
    if (problem) throw new InputError(problem);
    try {
      const created = await createPipeline(
        client,
        body,
        credentials.principal.userEntityRef,
      );
      res.status(201).json(created);
    } catch (e) {
      throw friendlyError(e, body);
    }
  });
}
