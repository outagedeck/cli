{
  description = "OutageDeck CLI for live cloud and SaaS provider status";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          outagedeck = pkgs.buildGoModule {
            pname = "outagedeck";
            version = "0.1.3";

            src = self;
            vendorHash = null;
            subPackages = [ "cmd/outagedeck" ];

            ldflags = [
              "-s"
              "-w"
              "-X main.version=0.1.3"
            ];

            meta = {
              description = "Check live cloud and SaaS provider status from a terminal or CI script";
              homepage = "https://outagedeck.com";
              license = pkgs.lib.licenses.mit;
              mainProgram = "outagedeck";
            };
          };
        in
        {
          inherit outagedeck;
          default = outagedeck;
        }
      );

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/outagedeck";
          meta.description = "Check live cloud and SaaS provider status";
        };
      });
    };
}
