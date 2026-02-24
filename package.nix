{
  name,
  version,
  lib,
  buildGoModule,
}:

buildGoModule {
  pname = name;
  inherit version;
  src = ./.;
  subPackages = [ "cmd/..." ];
  vendorHash = "sha256-sJlzlja7v4Db9B1GUBK1ISvKBdu6lzOSpd3wSSQPxJQ=";

  meta = with lib; {
    description = "Koptan, your autonomous autopilot for your cluster.";
    homepage = "https://koptan.felukka.org";
    platforms = platforms.linux;
  };
}
