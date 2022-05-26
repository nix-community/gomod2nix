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

let
  pythonEnv = pkgs.python3.withPackages (_: [ ]);

in
pkgs.mkShell {
  buildInputs = [
    pkgs.nix-prefetch-git
    pkgs.nixpkgs-fmt
    pkgs.go
    pkgs.gomod2nix
    pythonEnv
  ];
}
