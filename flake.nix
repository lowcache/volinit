{
  description = "Go development environment: toolchain, LSP, and a guarded module build";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      # =====================================================================
      # Project knobs.
      # =====================================================================

      # Module name, used only to name the built binary.
      pname = "volinit";

      # Hash over the module dependencies. `null` is correct here *because* the
      # dependencies are vendored in-tree: buildGoModule sees `vendor/` and
      # builds straight from it, fetching nothing. Do not set a hash; setting
      # one abandons the vendor path and reintroduces a network fetch. Deps
      # change via `go mod vendor` + committing `vendor/`, not via this value.
      vendorHash = null;

      # Extra tools beyond the go toolchain, as nixpkgs attribute names.
      extraTools = [
        "gopls"
        "go-tools"
      ];

      # Offline gates for `nix flake check`. See the shared note in the README:
      # one derivation per entry, empty by default.
      #   checkCommands = { vet = "go vet ./..."; test = "go test ./..."; };
      #
      # Go gates need the module cache, which the sandbox has no network for.
      # Vendor deps (`go mod vendor`) or keep gates to dependency-free packages.
      checkCommands = {
        vet = "go vet ./...";
        test = "go test ./...";
      };

      # =====================================================================

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      toolchain = pkgs: [ pkgs.go ] ++ map (name: pkgs.${name}) extraTools;

      mkChecks =
        pkgs:
        builtins.mapAttrs (
          name: command:
          pkgs.runCommand "check-${name}"
            {
              nativeBuildInputs = toolchain pkgs;
            }
            ''
              cp -r ${./.} src
              chmod -R u+w src
              cd src
              export GOFLAGS=-mod=vendor
              # The check sandbox has no C compiler; cgo defaults on and would
              # fail to build runtime/cgo. Nothing here needs it.
              export CGO_ENABLED=0
              export GOCACHE=$TMPDIR/go-build
              export GOPATH=$TMPDIR/go
              ${command}
              touch $out
            ''
        ) checkCommands;
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = toolchain pkgs;

          # Keep the module and build caches in-tree so the project stays
          # disposable and nothing lands in ~/go.
          shellHook = ''
            export GOPATH="$PWD/.go"
            export GOMODCACHE="$GOPATH/pkg/mod"
            export GOBIN="$GOPATH/bin"
            export PATH="$GOBIN:$PATH"
          '';
        };
      });

      checks = forAllSystems mkChecks;

      packages = forAllSystems (
        pkgs:
        nixpkgs.lib.optionalAttrs (builtins.pathExists ./go.mod) {
          default = pkgs.buildGoModule {
            inherit pname vendorHash;
            version = "0.0.0";
            src = ./.;
          };
        }
      );
    };
}
