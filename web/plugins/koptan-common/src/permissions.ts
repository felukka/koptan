import { createPermission } from '@backstage/plugin-permission-common';

export const koptanPipelineCreatePermission = createPermission({
  name: 'koptan.pipeline.create',
  attributes: { action: 'create' },
});

export const koptanPermissions = [koptanPipelineCreatePermission];
