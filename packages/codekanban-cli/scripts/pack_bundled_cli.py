#!/usr/bin/env python3
from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
CLI_DIR = ROOT / 'packages' / 'codekanban-cli'
SDK_DIR = ROOT / 'packages' / 'node-sdk'
CLI_COPY_ITEMS = ['bin', 'src', 'skills', 'README.md', 'package.json']


def npm_command() -> str:
    if os.name == 'nt':
        return shutil.which('npm.cmd') or shutil.which('npm') or 'npm.cmd'
    return shutil.which('npm') or 'npm'


def run(cmd: list[str], cwd: Path) -> None:
    subprocess.run(cmd, cwd=cwd, check=True)


def copy_cli_source(stage_dir: Path) -> None:
    for item_name in CLI_COPY_ITEMS:
        source = CLI_DIR / item_name
        target = stage_dir / item_name
        if source.is_dir():
            shutil.copytree(source, target)
        else:
            shutil.copy2(source, target)


def build_sdk_tarball(work_dir: Path) -> Path:
    npm = npm_command()
    sdk_out = work_dir / 'sdk-pack'
    sdk_out.mkdir(parents=True, exist_ok=True)
    run([npm, 'pack', '--pack-destination', str(sdk_out)], SDK_DIR)
    matches = sorted(sdk_out.glob('*.tgz'))
    if len(matches) != 1:
        raise RuntimeError(f'expected exactly one SDK tarball in {sdk_out}, found {len(matches)}')
    return matches[0]


def install_bundled_sdk(stage_dir: Path, sdk_tarball: Path) -> None:
    npm = npm_command()
    run([
        npm,
        'install',
        '--no-package-lock',
        '--omit=dev',
        '--no-save',
        str(sdk_tarball),
    ], stage_dir)


def pack_cli_bundle(output_dir: Path | None = None, dry_run: bool = False) -> Path | None:
    npm = npm_command()
    with tempfile.TemporaryDirectory(prefix='codekanban-cli-pack-') as temp_dir_raw:
        temp_dir = Path(temp_dir_raw)
        stage_dir = temp_dir / 'cli-stage'
        stage_dir.mkdir(parents=True, exist_ok=True)

        copy_cli_source(stage_dir)
        sdk_tarball = build_sdk_tarball(temp_dir)
        install_bundled_sdk(stage_dir, sdk_tarball)

        if dry_run:
            run([npm, 'pack', '--dry-run'], stage_dir)
            return None

        if output_dir is None:
            raise ValueError('output_dir is required when dry_run is false')
        output_dir.mkdir(parents=True, exist_ok=True)
        run([npm, 'pack', '--pack-destination', str(output_dir)], stage_dir)
        matches = sorted(output_dir.glob('*.tgz'))
        if not matches:
            raise RuntimeError(f'no CLI tarball created in {output_dir}')
        return matches[-1]


def main() -> None:
    import argparse

    parser = argparse.ArgumentParser(description='Pack @codekanban/cli with its bundled SDK dependency.')
    parser.add_argument('--dry-run', action='store_true', help='Run npm pack --dry-run for the staged CLI package')
    parser.add_argument('--pack-destination', type=Path, help='Directory that receives the packed CLI tarball')
    args = parser.parse_args()

    pack_cli_bundle(output_dir=args.pack_destination, dry_run=args.dry_run)


if __name__ == '__main__':
    main()
