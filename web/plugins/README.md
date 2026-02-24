# Plugins

- `koptan-common`: shared types and the `koptan.pipeline.create` permission.
- `koptan-backend`: reads Service, CI and CD resources (`koptan.felukka.org/v1`) and creates Services through the Kubernetes API.
- `koptan-react`: shared frontend code (API hook, `Phase`, `KoptanPage`, `EmptyPlaceholder`, `Dim`, `Badge`, `Icon`).
- `koptan-bay`, `koptan-raseef`, `koptan-scan-bay`, `koptan-signal-mast`: one frontend plugin per page.
- `infrastructure`, `monitoring`: placeholder pages using `EmptyPlaceholder` — wired until real backends are connected.
