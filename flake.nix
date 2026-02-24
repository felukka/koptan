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
