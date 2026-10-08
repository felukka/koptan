import type {
  HttpAuthService,
  PermissionsService,
} from '@backstage/backend-plugin-api';
import { InputError } from '@backstage/errors';
import { AuthorizeResult } from '@backstage/plugin-permission-common';
import {
  type CreatePipelineRequest,
  koptanPipelineCreatePermission,
} from '@internal/plugin-koptan-common';
import express from 'express';
import Router from 'express-promise-router';
import type { KoptanClient } from './crdClient';
import {
  buildActivity,
  buildOverview,
  buildPipelines,
  createPipeline,
  friendlyError,
  loadAll,
  validateCreate,
} from './pipelines';

export async function createRouter(options: {
  client: KoptanClient;
  httpAuth: HttpAuthService;
  permissions: PermissionsService;
  namespace?: string;
}): Promise<express.Router> {
  const { client, httpAuth, permissions, namespace } = options;
  const router = Router();
  router.use(express.json());

  router.get('/overview', async (_req, res) => {
    const { apps, slipways, voyages } = await loadAll(client, namespace);
    res.json(buildOverview(apps, slipways, voyages));
  });

  router.get('/pipelines', async (_req, res) => {
    const { apps, slipways, voyages } = await loadAll(client, namespace);
    res.json(buildPipelines(apps, slipways, voyages));
  });

  router.get('/activity', async (_req, res) => {
    const { apps, slipways, voyages } = await loadAll(client, namespace);
    res.json(buildActivity(apps, slipways, voyages));
  });

  router.get('/cluster', async (_req, res) => {
    res.json(await client.clusterInfo());
  });

  router.post('/pipelines', async (req, res) => {
    const credentials = await httpAuth.credentials(req, { allow: ['user'] });
    const [decision] = await permissions.authorize(
      [{ permission: koptanPipelineCreatePermission }],
      { credentials },
    );
    if (decision.result !== AuthorizeResult.ALLOW) {
      res
        .status(403)
        .json({ error: { name: 'NotAllowedError', message: 'Not allowed' } });
      return;
    }
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

  return router;
}
