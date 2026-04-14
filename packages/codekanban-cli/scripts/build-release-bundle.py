#!/usr/bin/env python3
from __future__ import annotations

import json
import shutil
import stat
import tarfile
import zipfile
from pathlib import Path

from pack_bundled_cli import pack_cli_bundle

ROOT = Path(__file__).resolve().parents[3]
CLI_DIR = ROOT / 'packages' / 'codekanban-cli'
SKILLS_DIR = CLI_DIR / 'skills'
RELEASE_TEMPLATE_DIR = CLI_DIR / 'release'
OUTPUT_DIR = ROOT / 'artifacts' / 'releases'


def read_package_version(package_dir: Path) -> str:
    return json.loads((package_dir / 'package.json').read_text(encoding='utf-8'))['version']


def clean_path(target: Path) -> None:
    if target.is_dir():
        shutil.rmtree(target)
    elif target.exists():
        target.unlink()


def render_template(source: Path, target: Path, replacements: dict[str, str], newline: str) -> None:
    text = source.read_text(encoding='utf-8')
    for key, value in replacements.items():
        text = text.replace(key, value)
    target.write_text(text, encoding='utf-8', newline=newline)


def ensure_unix_script(path: Path) -> None:
    data = path.read_bytes()
    if b'\r' in data:
        raise RuntimeError(f'{path} still contains CR characters')
    if not data.startswith(b'#!'):
        raise RuntimeError(f'{path} is missing a shebang')
    path.chmod(path.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


def archive_mode(item: Path, is_dir: bool) -> int:
    if is_dir:
        return 0o755
    if item.suffix == '.sh':
        return 0o755
    return 0o644


def add_directory_to_zip(zip_file: zipfile.ZipFile, directory: Path, prefix: str) -> None:
    for item in sorted(directory.rglob('*')):
        arcname = Path(prefix) / item.relative_to(directory)
        if item.is_dir():
            info = zipfile.ZipInfo(f'{arcname.as_posix()}/')
            info.external_attr = (archive_mode(item, True) & 0xFFFF) << 16
            zip_file.writestr(info, '')
            continue
        info = zipfile.ZipInfo(arcname.as_posix())
        info.external_attr = (archive_mode(item, False) & 0xFFFF) << 16
        zip_file.writestr(info, item.read_bytes(), compress_type=zipfile.ZIP_DEFLATED)


def add_directory_to_tar(tar_file: tarfile.TarFile, directory: Path, prefix: str) -> None:
    for item in sorted(directory.rglob('*')):
        arcname = f"{prefix}/{item.relative_to(directory).as_posix()}"
        info = tar_file.gettarinfo(str(item), arcname=arcname)
        info.mode = archive_mode(item, item.is_dir())
        if item.is_dir():
            tar_file.addfile(info)
            continue
        with item.open('rb') as handle:
            tar_file.addfile(info, handle)


def main() -> None:
    cli_version = read_package_version(CLI_DIR)
    bundle_name = f'codekanban-cli-bundle-v{cli_version}'
    bundle_dir = OUTPUT_DIR / bundle_name
    zip_path = OUTPUT_DIR / f'{bundle_name}.zip'
    tar_path = OUTPUT_DIR / f'{bundle_name}.tar.gz'

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    clean_path(bundle_dir)
    clean_path(zip_path)
    clean_path(tar_path)

    npm_dir = bundle_dir / 'npm'
    skills_out = bundle_dir / 'skills'
    npm_dir.mkdir(parents=True, exist_ok=True)
    shutil.copytree(SKILLS_DIR, skills_out)

    pack_cli_bundle(output_dir=npm_dir)

    replacements = {
        '__CLI_VERSION__': cli_version,
    }
    for template in RELEASE_TEMPLATE_DIR.iterdir():
        target = bundle_dir / template.name
        newline = '\n' if template.suffix == '.sh' else '\r\n' if template.suffix == '.cmd' else '\n'
        render_template(template, target, replacements, newline)
        if template.suffix == '.sh':
            ensure_unix_script(target)

    with zipfile.ZipFile(zip_path, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
        add_directory_to_zip(archive, bundle_dir, bundle_name)

    with tarfile.open(tar_path, 'w:gz') as archive:
        root_info = archive.gettarinfo(str(bundle_dir), arcname=bundle_name)
        root_info.mode = 0o755
        archive.addfile(root_info)
        add_directory_to_tar(archive, bundle_dir, bundle_name)

    print(json.dumps({
        'bundle_dir': str(bundle_dir),
        'zip': str(zip_path),
        'tar_gz': str(tar_path),
    }, indent=2))


if __name__ == '__main__':
    main()
