#!/usr/bin/env python3
import os
import pathlib
import subprocess
import sys

script = os.environ.get("CHARON_RUN_SCRIPT")
if not script:
    script = str(pathlib.Path(__file__).resolve().parent.parent / "scripts" / "run.sh")
result = subprocess.run(["/bin/sh", script])
sys.exit(result.returncode)
