"""Build the deployment tarball for new-api v1.2.5 (working tree, build artefacts excluded)."""

import os
import sys
import tarfile

ROOT = r"D:\newapi"
OUT = r"D:\newapi\deploy_v1.2.5.tar.gz"

EXCLUDE_DIRS = {
    ".git",
    ".github",
    ".ops",
    ".vscode",
    "bin",
    "node_modules",
    "dist",
    ".gocache",
    "__pycache__",
    ".venv",
    "data",
    "logs",
    "tmp_admin",
    "tmp_ask",
    "tmp_test",
}
EXCLUDE_SUFFIXES = (".tar.gz", ".log", ".db", ".sqlite", ".exe", ".zip")
# Compiled gateways and packaged artefacts living in the working tree.
EXCLUDE_BASENAMES = {
    "new-api",
    "new-api-current",
    "new-api-linux-amd64",
    "new-api-linux-arm64",
    "newapi-linux-amd64",
}


def main():
    if os.path.exists(OUT):
        os.remove(OUT)
    added = 0
    with tarfile.open(OUT, "w:gz", compresslevel=6) as tar:
        for dirpath, dirnames, filenames in os.walk(ROOT):
            dirnames[:] = sorted(
                name
                for name in dirnames
                if name not in EXCLUDE_DIRS and not name.startswith(".git")
            )
            for filename in sorted(filenames):
                if filename.endswith(EXCLUDE_SUFFIXES):
                    continue
                if filename in EXCLUDE_BASENAMES:
                    continue
                path = os.path.join(dirpath, filename)
                arcname = os.path.relpath(path, ROOT)
                tar.add(path, arcname=arcname)
                added += 1
    size = os.path.getsize(OUT) / (1024 * 1024)
    print(f"{added} files -> {OUT} ({size:.1f} MiB)")


if __name__ == "__main__":
    sys.exit(main())
