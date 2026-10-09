# Plugins

- `koptan-common`: shared types and the `koptan.pipeline.create` permission.
- `koptan-backend`: reads Service, CI and CD resources (`koptan.felukka.org/v1`) and creates Services through the Kubernetes API. `src/service/k8s` is the cluster client, `src/service/pipelines` the Service -> CI -> CD views, and `src/service/routes` one module per route family.
- `koptan-react`: shared frontend code (API hook, `Phase`, `KoptanPage`) and the Minzar stylesheet in `src/styles/`.
- `koptan-bay`, `koptan-raseef`, `koptan-scan-bay`, `koptan-signal-mast`: one frontend plugin per page.
