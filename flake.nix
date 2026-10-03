{
  description = "Koptan, your autonomous autopilot for your cluster.";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-26.05";
  };

  outputs =
    { self, nixpkgs }:
    let
      name = "koptan";
      version = "0.1";
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
    in
    {
      packages = nixpkgs.lib.genAttrs systems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.buildGoModule {
            pname = name;
            inherit version;
            src = ./.;
            subPackages = [ "cmd/..." ];
            vendorHash = "sha256-sJlzlja7v4Db9B1GUBK1ISvKBdu6lzOSpd3wSSQPxJQ=";

            meta = with pkgs.lib; {
              description = "Koptan Kubernetes Operator";
              homepage = "https://felukka.org";
              platforms = platforms.linux;
            };
          };

          docker = pkgs.dockerTools.buildImage {
            name = "${name}";
            tag = version;
            copyToRoot = pkgs.buildEnv {
              name = "image-root";
              paths = [ self.packages.${system}.default ];
              pathsToLink = [ "/bin" ];
            };
            config = {
              Cmd = [ "/bin/koptan" ];
            };
          };

          web = pkgs.mkDerivation {
            pname = "${name}-ui";
            inherit version;
            src = ./web;

            nativeBuildInputs = with pkgs; [
              nodejs
              yarn
            ];

            buildPhase = ''
              export HOME=$TMPDIR
              yarn --immutable --immutable-cache
              yarn tsc
              yarn build:all
            '';

            installPhase = ''
              mkdir -p $out/share/koptan-ui
              cp -r . $out/share/koptan-ui/
              mkdir -p $out/bin
              cat << 'EOF' > $out/bin/koptan-ui
              #!/bin/sh
              cd @out@/share/koptan-ui
              exec node packages/backend/dist/index.cjs.js "$@"
              EOF
              substituteInPlace $out/bin/koptan-ui --subst-var out
              chmod +x $out/bin/koptan-ui
            '';
          };

          web-docker = pkgs.dockerTools.buildImage {
            name = "${name}-web";
            tag = version;
            copyToRoot = pkgs.buildEnv {
              name = "image-root";
              paths = [
                self.packages.${system}.web
                pkgs.nodejs
              ];
              pathsToLink = [
                "/bin"
                "/share"
              ];
            };
            config = {
              Cmd = [ "/bin/koptan-ui" ];
              WorkingDir = "/share/koptan-ui";
              ExposedPorts = {
                "3000/tcp" = { };
              };
            };
          };
        }
      );

      devShells = nixpkgs.lib.genAttrs systems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShellNoCC {
            buildInputs = with pkgs; [
              gcc
              gnumake
              go
              gofumpt
              goimports-reviser
              golangci-lint
              golines
              gopls
              go-tools
              gotools
              kubebuilder
              nodejs_22
              yarn
              typescript
              typescript-language-server
              biome
              (python3.withPackages (
                p: with p; [
                  mkdocs-material
                ]
              ))
            ];
          };
        }
      );
    };
}
