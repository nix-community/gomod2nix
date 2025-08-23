{
  stdenv,
  buildGoApplication,
  fetchFromGitHub,
  go-bindata,
  installShellFiles,
  pkg-config,
  which,
  libvirt,
  darwin,
}:

buildGoApplication rec {
  pname = "minikube";
  version = "1.15.0";

  modules = ./gomod2nix.toml;

  doCheck = false;

  src = fetchFromGitHub {
    owner = "kubernetes";
    repo = "minikube";
    rev = "v${version}";
    sha256 = "1n1jhsa0lrfpqvl7m5il37l3f22ffgg4zv8g42xq24cgna951a1z";
  };

  nativeBuildInputs = [
    go-bindata
    installShellFiles
    pkg-config
    which
  ];

  buildInputs =
    if stdenv.isDarwin then
      [ darwin.apple_sdk.frameworks.vmnet ]
    else if stdenv.isLinux then
      [ libvirt ]
    else
      null;

  buildPhase = ''
    make COMMIT=${src.rev}
  '';

  installPhase = ''
    install out/minikube -Dt $out/bin

    export HOME=$PWD
    export MINIKUBE_WANTUPDATENOTIFICATION=false
    export MINIKUBE_WANTKUBECTLDOWNLOADMSG=false

    for shell in bash zsh fish; do
      $out/bin/minikube completion $shell > minikube.$shell
      installShellCompletion minikube.$shell
    done
  '';

}
