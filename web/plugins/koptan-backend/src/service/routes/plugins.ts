import type { Router } from 'express';
import { loadScanBay } from '../plugins';
import { type RouteContext, read } from './context';

/** CIPlugins and the latest results of their steps, for the Scan Bay. */
export function pluginRoutes(router: Router, ctx: RouteContext) {
  router.get('/plugins', async (_req, res) => {
    res.json(await read(() => loadScanBay(ctx.client, ctx.namespace)));
  });
}
