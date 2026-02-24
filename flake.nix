{
  description = "Koptan";

  inputs.nixpkgs.url = "nixpkgs/nixos-25.11";

  outputs =
    { self, nixpkgs }:
    let
      version = builtins.substring 0 8 (self.lastModifiedDate or self.lastModified or "19700101");
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
    in
    {
      packages = nixpkgs.lib.genAttrs systems (system: let pkgs = import nixpkgs { inherit system; }; in {
        default = pkgs.buildGoModule {
          pname = "koptan";
          inherit version;
          src = ./.;
          subPackages = [ "cmd/..." ];
          vendorHash = "sha256-tKf1DkA4RAfmQA+4SSPi80BCV5bdFZCWbXuBzU4Ogdk=";
        };
      });
      devShells = nixpkgs.lib.genAttrs systems (system: let pkgs = import nixpkgs { inherit system; }; in {
        default = pkgs.mkShellNoCC {
          buildInputs = with pkgs; [
            gcc
            gnumake
            go
            go-tools
            gopls
            gotools
            kubebuilder
          ];
        };
      });
    };
}
