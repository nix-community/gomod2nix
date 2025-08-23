{
  pkgs ? import <nixpkgs> { },
}:

{
  helm = pkgs.callPackage ./helm { };
  linkerd = pkgs.callPackage ./linkerd { };
  minikube = pkgs.callPackage ./minikube { };
}
