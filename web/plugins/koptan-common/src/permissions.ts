import { createPermission } from '@backstage/plugin-permission-common';

export const koptanPipelineCreatePermission = createPermission({
  name: 'koptan.pipeline.create',
  attributes: { action: 'create' },
});

export const koptanAlertCreatePermission = createPermission({
  name: 'koptan.alert.create',
  attributes: { action: 'create' },
});

export const koptanSelfServiceCreatePermission = createPermission({
  name: 'koptan.selfservice.create',
  attributes: { action: 'create' },
});

/** Sending a prompt changes code and deploys it, so it has its own permission. */
export const koptanSelfServicePromptPermission = createPermission({
  name: 'koptan.selfservice.prompt',
  attributes: { action: 'update' },
});

export const koptanPermissions = [
  koptanPipelineCreatePermission,
  koptanAlertCreatePermission,
  koptanSelfServiceCreatePermission,
  koptanSelfServicePromptPermission,
];
