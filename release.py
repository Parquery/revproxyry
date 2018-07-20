#!/usr/bin/env python3
"""
releases the code to the given directory as a binary package and a debian package.

The architecture is assumed to be AMD64 (i.e. Linux x64). If you want to release the code for a different architecture,
then please do that manually.
"""

import argparse
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile


def main() -> int:
    """ 
    executes the main routine. 
    """
    parser = argparse.ArgumentParser()
    parser.add_argument("--release_dir", help="directory where to put the release", required=True)
    args = parser.parse_args()

    release_dir = pathlib.Path(args.release_dir)

    release_dir.mkdir(exist_ok=True, parents=True)

    # set the working directory to the script's directory
    script_dir = pathlib.Path(os.path.dirname(os.path.realpath(__file__)))

    subprocess.check_call(["go", "install", "./..."], cwd=script_dir.as_posix())

    if "GOPATH" not in os.environ:
        raise RuntimeError("Expected variable GOPATH in the environment")

    gopath_val = os.environ["GOPATH"]
    gopaths = os.environ["GOPATH"].split(os.pathsep)

    if not gopaths:
        raise RuntimeError("Expected at least a directory in GOPATH, but got none")

    # main gopath
    gopath = pathlib.Path(gopaths[0])
    go_bin_dir = gopath / "bin"

    bin_path = go_bin_dir / "revproxyry"

    # get gopath version
    version = subprocess.check_output([bin_path.as_posix(), "--version"], universal_newlines=True).strip()

    # release the binary package
    with tempfile.TemporaryDirectory() as tmp_dir:
        bin_package_dir = pathlib.Path(tmp_dir) / "revproxyry-{}-linux-x64".format(version)

        target = bin_package_dir / "bin/revproxyry".format(version)
        target.parent.mkdir(parents=True)

        shutil.copy(bin_path.as_posix(), target.as_posix())

        tar_path = bin_package_dir.parent / "revproxyry-{}-linux-x64.tar.gz".format(version)

        subprocess.check_call([
            "tar", "-czf", tar_path.as_posix(), "revproxyry-{}-linux-x64".format(version)],
            cwd=bin_package_dir.parent.as_posix())

        shutil.move(tar_path.as_posix(), (release_dir / tar_path.name).as_posix())

    # release the debian package
    with tempfile.TemporaryDirectory() as tmp_dir:
        deb_package_dir = pathlib.Path(tmp_dir) / "revproxyry_{}_amd64".format(version)

        target = deb_package_dir / "usr/bin/revproxyry".format(version)
        target.parent.mkdir(parents=True)
        shutil.copy(bin_path.as_posix(), target.as_posix())

        control_pth = deb_package_dir / "DEBIAN/control"
        control_pth.parent.mkdir(parents=True)

        control_pth.write_text("\n".join([
            "Package: revproxyry",
            "Version: {}".format(version),
            "Maintainer: Marko Ristin (marko@parquery.com)",
            "Architecture: amd64",
            "Description: revproxyry is a reverse proxy with integrated Let's encrypt client that "
            "automatically renews SSL certificates.",
            ""]))

        subprocess.check_call(["dpkg-deb", "--build", deb_package_dir.as_posix()],
                              cwd=deb_package_dir.parent.as_posix(),
                              stdout=subprocess.DEVNULL)

        deb_pth = deb_package_dir.parent / "revproxyry_{}_amd64.deb".format(version)

        shutil.move(deb_pth.as_posix(), (release_dir / deb_pth.name).as_posix())

    print("Released to: {}".format(release_dir))

    return 0


if __name__ == "__main__":
    sys.exit(main())
