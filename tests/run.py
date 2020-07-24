#!/usr/bin/env python
import subprocess
import os.path
import sys


if __name__ == '__main__':
    script_dir = os.path.dirname(os.path.abspath(__file__))
    root_dir = os.path.dirname(script_dir)

    cmd = os.path.join(root_dir, "gomod2nix")

    def run(directory):
        print(f"Running {directory}")

        subprocess.run([cmd, "--dir", directory, "--outdir", directory], check=True)

        build_expr = ("""
        with (import <nixpkgs> { overlays = [ (import %s/overlay.nix) ]; }); callPackage %s {}"
        """.replace("\n", " ") % (root_dir, directory))
        subprocess.run(["nix-build", "--expr", build_expr], check=True)

    for f in os.listdir(script_dir):

        d = os.path.join(script_dir, f)
        if os.path.isdir(d):
            try:
                run(d)
            except Exception:
                sys.stderr.write(f"Error running {d}\n")
