import { InputError } from '@backstage/errors';
import {
  type CreateAlertRequest,
  koptanAlertCreatePermission,
} from '@internal/plugin-koptan-common';
import type { Router } from 'express';
import { createAlert, loadSignalMast, validateCreateAlert } from '../alerts';
import { friendlyError } from '../k8s';
import { authorize, type RouteContext, read } from './context';

/** Alerts and their deliveries, for the Signal Mast. */
export function alertRoutes(router: Router, ctx: RouteContext) {
  router.get('/alerts', async (_req, res) => {
    res.json(await read(() => loadSignalMast(ctx.client, ctx.namespace)));
  });

  router.post('/alerts', async (req, res) => {
    const credentials = await authorize(ctx, req, koptanAlertCreatePermission);
    const body = req.body as CreateAlertRequest;
    const problem = validateCreateAlert(body);
    if (problem) throw new InputError(problem);
    try {
      res
        .status(201)
        .json(
          await createAlert(
            ctx.client,
            body,
            credentials.principal.userEntityRef,
          ),
        );
    } catch (e) {
      throw friendlyError(e, body);
    }
  });
}
