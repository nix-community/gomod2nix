{
  stdenv,
  buildGoApplication,
  fetchFromGitHub,
  installShellFiles,
}:

buildGoApplication rec {
  pname = "helm";
  version = "3.4.1";

  src = fetchFromGitHub {
    owner = "helm";
    repo = "helm";
    rev = "v${version}";
    sha256 = "13w0s11319qg9mmmxc24mlj0hrp0r529p3ny4gfzsl0vn3qzd6i2";
  };
  modules = ./gomod2nix.toml;

  doCheck = false;

  subPackages = [ "cmd/helm" ];
  buildFlagsArray = [ "-ldflags=-w -s -X helm.sh/helm/v3/internal/version.version=v${version}" ];

  nativeBuildInputs = [ installShellFiles ];
  postInstall = ''
    $out/bin/helm completion bash > helm.bash
    $out/bin/helm completion zsh > helm.zsh
    installShellCompletion helm.{bash,zsh}
  '';
}
