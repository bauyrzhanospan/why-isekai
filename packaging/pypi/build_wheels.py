#!/usr/bin/env python3
"""Build PyPI wheels that carry the prebuilt isekai binary (the ruff scheme).

Each wheel is platform-specific and contains:
  why_isekai-<v>.data/scripts/isekai[.exe]   pip puts it on PATH as `isekai`
  why_isekai/                                `why-isekai` entry point and
                                             `python -m why_isekai`, both exec the binary

  python3 build_wheels.py --version 0.1.0 --dist ../../dist --out out
  python3 build_wheels.py --self-test

Standard library only.
"""
import argparse
import base64
import hashlib
import io
import os
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
import zipfile

NAME = "why-isekai"
DIST_NAME = "why_isekai"
REPO = "https://github.com/bauyrzhanospan/why-isekai"

# GoReleaser os/arch -> wheel platform tag. The binary is static
# (CGO_ENABLED=0), so it runs on glibc and musl alike.
PLATFORMS = {
    ("linux", "amd64"): "manylinux_2_17_x86_64.manylinux2014_x86_64.musllinux_1_1_x86_64",
    ("linux", "arm64"): "manylinux_2_17_aarch64.manylinux2014_aarch64.musllinux_1_1_aarch64",
    ("darwin", "amd64"): "macosx_10_12_x86_64",
    ("darwin", "arm64"): "macosx_11_0_arm64",
    ("windows", "amd64"): "win_amd64",
    ("windows", "arm64"): "win_arm64",
}

INIT_PY = '''"""why-isekai: project memory for AI agents, kept in git."""
import os
import shutil
import sys
import sysconfig


def find_isekai_bin() -> str:
    """Return the path of the isekai binary installed with this package."""
    exe = "isekai" + (".exe" if sys.platform == "win32" else "")
    candidates = [os.path.join(sysconfig.get_path("scripts"), exe)]
    for scheme in (f"{os.name}_user", "osx_framework_user"):
        try:
            candidates.append(os.path.join(sysconfig.get_path("scripts", scheme), exe))
        except KeyError:
            pass
    for c in candidates:
        if os.path.isfile(c):
            return c
    found = shutil.which(exe)
    if found:
        return found
    raise FileNotFoundError(f"isekai binary not found in {candidates}")
'''

MAIN_PY = '''import os
import subprocess
import sys

from why_isekai import find_isekai_bin


def main() -> None:
    exe = find_isekai_bin()
    if sys.platform == "win32":
        sys.exit(subprocess.call([exe, *sys.argv[1:]]))
    os.execv(exe, [exe, *sys.argv[1:]])


if __name__ == "__main__":
    main()
'''


def record_line(path: str, data: bytes) -> str:
    digest = base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()
    return f"{path},sha256={digest},{len(data)}"


def metadata(version: str) -> str:
    readme = ""
    root_readme = os.path.join(os.path.dirname(__file__), "..", "..", "README.md")
    if os.path.exists(root_readme):
        with open(root_readme, encoding="utf-8") as f:
            readme = f.read()
    return (
        "Metadata-Version: 2.1\n"
        f"Name: {NAME}\n"
        f"Version: {version}\n"
        "Summary: Project memory for AI agents, kept in git: Why/Tried/Next/Done/Agent commit trailers, secrets by reference, project wiki.\n"
        f"Home-page: {REPO}\n"
        "License: MIT\n"
        "Keywords: ai-agents,git,commit-trailers,agents-md,llm\n"
        "Classifier: License :: OSI Approved :: MIT License\n"
        "Classifier: Topic :: Software Development :: Version Control :: Git\n"
        "Requires-Python: >=3.8\n"
        "Description-Content-Type: text/markdown\n"
        "\n" + readme
    )


def build_wheel(version: str, binary: bytes, exe: str, tag: str, out: str) -> str:
    dist_info = f"{DIST_NAME}-{version}.dist-info"
    data_dir = f"{DIST_NAME}-{version}.data"
    files = {
        f"{DIST_NAME}/__init__.py": INIT_PY.encode(),
        f"{DIST_NAME}/__main__.py": MAIN_PY.encode(),
        f"{data_dir}/scripts/{exe}": binary,
        f"{dist_info}/METADATA": metadata(version).encode(),
        f"{dist_info}/WHEEL": (
            "Wheel-Version: 1.0\nGenerator: why-isekai build_wheels.py\nRoot-Is-Purelib: false\n"
            + "".join(f"Tag: py3-none-{t}\n" for t in tag.split("."))
        ).encode(),
        f"{dist_info}/entry_points.txt": b"[console_scripts]\nwhy-isekai = why_isekai.__main__:main\n",
    }
    record_path = f"{dist_info}/RECORD"
    record = "\n".join(record_line(p, d) for p, d in files.items()) + f"\n{record_path},,\n"
    files[record_path] = record.encode()
    os.makedirs(out, exist_ok=True)
    wheel = os.path.join(out, f"{DIST_NAME}-{version}-py3-none-{tag}.whl")
    with zipfile.ZipFile(wheel, "w", zipfile.ZIP_DEFLATED) as z:
        for p, d in files.items():
            info = zipfile.ZipInfo(p, date_time=(2026, 1, 1, 0, 0, 0))
            info.external_attr = (0o100755 if p.endswith(exe) and "/scripts/" in p else 0o100644) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(info, d)
    return wheel


def read_binary(archive: str, exe: str) -> bytes:
    if archive.endswith(".zip"):
        with zipfile.ZipFile(archive) as z:
            return z.read(exe)
    with tarfile.open(archive) as t:
        f = t.extractfile(exe)
        if f is None:
            raise SystemExit(f"{exe} not in {archive}")
        return f.read()


def build_all(version: str, dist: str, out: str, only=None) -> list:
    shutil.rmtree(out, ignore_errors=True)
    wheels = []
    for (goos, goarch), tag in PLATFORMS.items():
        if only and (goos, goarch) != only:
            continue
        exe = "isekai.exe" if goos == "windows" else "isekai"
        ext = "zip" if goos == "windows" else "tar.gz"
        archive = os.path.join(dist, f"isekai_{version}_{goos}_{goarch}.{ext}")
        if not os.path.exists(archive):
            raise SystemExit(f"missing {archive}")
        wheels.append(build_wheel(version, read_binary(archive, exe), exe, tag, out))
    return wheels


def self_test() -> None:
    goos = {"linux": "linux", "darwin": "darwin"}.get(sys.platform)
    goarch = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine().lower())
    if not goos or not goarch:
        print("self-test skipped on this platform")
        return
    with tempfile.TemporaryDirectory() as tmp:
        dist = os.path.join(tmp, "dist")
        os.makedirs(dist)
        fake = b'#!/bin/sh\necho "fake isekai $*"\nexit 3\n'
        with tarfile.open(os.path.join(dist, f"isekai_9.9.9_{goos}_{goarch}.tar.gz"), "w:gz") as t:
            info = tarfile.TarInfo("isekai")
            info.size, info.mode = len(fake), 0o755
            t.addfile(info, io.BytesIO(fake))
        wheels = build_all("9.9.9", dist, os.path.join(tmp, "out"), only=(goos, goarch))
        venv = os.path.join(tmp, "venv")
        subprocess.check_call([sys.executable, "-m", "venv", venv])
        bindir = os.path.join(venv, "bin")
        subprocess.check_call([os.path.join(bindir, "python"), "-m", "pip", "install", "-q", "--no-index", wheels[0]])
        for cmd in (["isekai"], ["why-isekai"], ["python", "-m", "why_isekai"]):
            r = subprocess.run([os.path.join(bindir, cmd[0]), *cmd[1:], "brief"], capture_output=True, text=True)
            if r.returncode != 3 or r.stdout.strip() != "fake isekai brief":
                raise SystemExit(f"self-test failed for {cmd}: {r}")
    print("pypi packaging self-test ok")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--version")
    ap.add_argument("--dist", default=os.path.join(os.path.dirname(__file__), "..", "..", "dist"))
    ap.add_argument("--out", default=os.path.join(os.path.dirname(__file__), "out"))
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        self_test()
        return
    if not a.version:
        ap.error("--version is required")
    for w in build_all(a.version, a.dist, a.out):
        print(w)


if __name__ == "__main__":
    main()
