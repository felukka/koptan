# Plugins

- `koptan-common`: shared types and the `koptan.pipeline.create` permission.
- `koptan-backend`: reads Service, CI and CD resources (`koptan.felukka.org/v1`) and creates Services through the Kubernetes API.
- `koptan-react`: shared frontend code (API hook, `Phase`, `KoptanPage`).
- `koptan-bay`, `koptan-raseef`, `koptan-scan-bay`, `koptan-signal-mast`: one frontend plugin per page.
- `services`, `scanning`, `monitoring`, `metrics`, `infrastructure`, `alerting`, `koptanv1-common`: untouched generator scaffolds from main, not wired into the app yet.
