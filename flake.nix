{
  description = "Koptan, your autonomous autopilot for your cluster.";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-26.05";
  };

  outputs =
    { self, nixpkgs }:
    let
      name = "koptan";
      version = "0.0.1";

      inherit (nixpkgs) lib;

      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      overlay = final: _: {
        koptan = final.callPackage ./package.nix { inherit name version; };
      };

      pkgsFor = lib.genAttrs systems (
        system:
        import nixpkgs {
          inherit system;
          overlays = [ overlay ];
        }
      );

      forAllSystems = f: lib.genAttrs systems (system: f pkgsFor.${system});
    in
    {
      overlays.default = overlay;

      packages = forAllSystems (pkgs: {
        default = pkgs.koptan;

        docker = pkgs.dockerTools.buildImage {
          inherit name;
          tag = version;
          copyToRoot = pkgs.buildEnv {
            name = "image-root";
            paths = [ pkgs.koptan ];
            pathsToLink = [ "/bin" ];
          };
          config = {
            Cmd = [ "/bin/koptan" ];
          };
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gnumake
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
      });
    };
}
