import express from 'express';
import Router from 'express-promise-router';
import type { RouteContext } from './routes/context';
import { pipelineRoutes } from './routes/pipelines';
import { pluginRoutes } from './routes/plugins';

/** Mounts one route module per Koptan resource family. */
export async function createRouter(ctx: RouteContext): Promise<express.Router> {
  const router = Router();
  router.use(express.json());
  pipelineRoutes(router, ctx);
  pluginRoutes(router, ctx);
  return router;
}
