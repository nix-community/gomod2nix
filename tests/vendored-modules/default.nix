{
  runCommand,
  buildGoApplication,
  fetchFromGitHub,
}:

let
  version = "0.23.1";

in
buildGoApplication {
  pname = "dstask";
  inherit version;

  src = fetchFromGitHub {
    owner = "naggie";
    repo = "dstask";
    rev = "v${version}";
    sha256 = "0rfz8jim0xqcwdb5n28942v9r3hbvhjrwdgzvbwc9f9psqg2s8d2";
  };

  modules = null;

  ldflags = [
    "-w"
    "-s"
    "-X github.com/naggie/dstask.VERSION=${version}"
    "-X github.com/naggie/dstask.GIT_COMMIT=v${version}"
  ];

  subPackages = [ "cmd/dstask.go" ];
}
