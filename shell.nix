{ pkgs ? (
    let
      inherit (builtins) fromJSON readFile;
      flakeLock = fromJSON (readFile ./flake.lock);
      locked = flakeLock.nodes.nixpkgs.locked;
      nixpkgs = assert locked.type == "github"; builtins.fetchTarball {
        url = "https://github.com/${locked.owner}/${locked.repo}/archive/${locked.rev}.tar.gz";
        sha256 = locked.narHash;
      };
    in
    import nixpkgs {
      overlays = [
        (import ./overlay.nix)
      ];
    }
  )
}:

pkgs.mkShell {
  NIX_PATH = "nixpkgs=${builtins.toString pkgs.path}";
  buildInputs = [
    pkgs.nixpkgs-fmt
    pkgs.gomod2nix.go
    pkgs.gomod2nix
    pkgs.golangci-lint
  ];
}
