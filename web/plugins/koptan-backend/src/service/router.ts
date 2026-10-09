import express from 'express';
import Router from 'express-promise-router';
import type { RouteContext } from './routes/context';
import { pipelineRoutes } from './routes/pipelines';

/** Mounts one route module per Koptan resource family. */
export async function createRouter(ctx: RouteContext): Promise<express.Router> {
  const router = Router();
  router.use(express.json());
  pipelineRoutes(router, ctx);
  return router;
}
