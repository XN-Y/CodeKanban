#!/usr/bin/env python3
"""
NPM 发布构建脚本：构建多平台版本并准备 npm 发布
使用 go-npm 方式，构建所有平台的二进制文件到 bin 目录
"""
import os
import shutil
import subprocess
import sys
from pathlib import Path


def run_command(cmd: list[str], cwd: Path | None = None, shell: bool = False, env: dict = None) -> int:
    """执行命令并实时输出"""
    print(f"[执行] {' '.join(cmd) if not shell else cmd[0]}")
    if shell:
        result = subprocess.run(cmd[0], cwd=cwd, shell=True, env=env)
    else:
        result = subprocess.run(cmd, cwd=cwd, env=env)
    return result.returncode


def clean_static_dir(static_dir: Path):
    """清空 static 目录但保留 README.md"""
    print(f"[清理] 清空 {static_dir} 目录（保留 README.md）")

    if not static_dir.exists():
        static_dir.mkdir(parents=True)
        print(f"[创建] {static_dir} 目录")
        return

    for item in static_dir.iterdir():
        if item.name == "README.md":
            continue

        if item.is_file():
            item.unlink()
            print(f"[删除] {item}")
        elif item.is_dir():
            shutil.rmtree(item)
            print(f"[删除] {item}/")


def copy_dist_to_static(dist_dir: Path, static_dir: Path):
    """复制 ui/dist 到 static 目录"""
    print(f"[复制] {dist_dir} -> {static_dir}")

    if not dist_dir.exists():
        print(f"[错误] {dist_dir} 不存在，请先构建前端")
        return False

    for item in dist_dir.iterdir():
        dest = static_dir / item.name

        if item.is_file():
            shutil.copy2(item, dest)
            print(f"  复制文件: {item.name}")
        elif item.is_dir():
            if dest.exists():
                shutil.rmtree(dest)
            shutil.copytree(item, dest)
            print(f"  复制目录: {item.name}/")

    return True


def build_go_multiplatform(root_dir: Path):
    """构建多平台版本（用于 npm 发布）"""
    print("\n[步骤 3/3] 构建多平台 Go 程序")

    bin_dir = root_dir / "binary"
    bin_dir.mkdir(exist_ok=True)

    # go-npm 支持的平台和架构
    platforms = [
        ("linux", "amd64"),
        ("linux", "arm64"),
        ("darwin", "amd64"),
        ("darwin", "arm64"),
        ("windows", "amd64"),
    ]

    success_count = 0
    total_size = 0

    for goos, goarch in platforms:
        print(f"\n构建 {goos}/{goarch}...")

        # 输出文件名：codekanban（不带平台后缀，go-npm 会自动处理）
        output_name = "codekanban"
        if goos == "windows":
            output_name += ".exe"

        # 为不同平台创建子目录
        platform_dir = bin_dir / f"{goos}-{goarch}"
        platform_dir.mkdir(exist_ok=True)
        output_path = platform_dir / output_name

        # 设置环境变量
        env = os.environ.copy()
        env["GOOS"] = goos
        env["GOARCH"] = goarch
        env["CGO_ENABLED"] = "0"  # 禁用 CGO 以支持交叉编译

        build_cmd = [
            "go", "build",
            "-ldflags=-s -w",
            "-trimpath",
            "-o", str(output_path),
            "."
        ]

        result = subprocess.run(build_cmd, cwd=root_dir, env=env)

        if result.returncode != 0:
            print(f"[错误] {goos}/{goarch} 构建失败")
            return result.returncode

        # 输出文件大小
        if output_path.exists():
            size_mb = output_path.stat().st_size / (1024 * 1024)
            total_size += output_path.stat().st_size
            print(f"  OK: {output_name} ({size_mb:.2f} MB)")
            success_count += 1

    print(f"\n成功构建 {success_count}/{len(platforms)} 个平台")
    print(f"总大小: {total_size / (1024 * 1024):.2f} MB")
    print(f"输出目录: {bin_dir}")
    return 0


def main():
    # 获取项目根目录
    root_dir = Path(__file__).parent.absolute()
    ui_dir = root_dir / "ui"
    dist_dir = ui_dir / "dist"
    static_dir = root_dir / "static"

    print("=" * 60)
    print("NPM 发布构建（多平台模式）")
    print("=" * 60)

    # 步骤 1: 构建前端
    print("\n[步骤 1/3] 构建前端项目")
    if not ui_dir.exists():
        print(f"[错误] {ui_dir} 目录不存在")
        return 1

    # 根据操作系统选择是否使用 shell
    is_windows = sys.platform.startswith('win')

    if is_windows:
        ret = run_command(["pnpm build"], cwd=ui_dir, shell=True)
    else:
        ret = run_command(["pnpm", "build"], cwd=ui_dir)

    if ret != 0:
        print("[错误] 前端构建失败")
        return ret

    # 步骤 2: 复制产物到 static
    print("\n[步骤 2/3] 复制前端产物到 static 目录")
    clean_static_dir(static_dir)

    if not copy_dist_to_static(dist_dir, static_dir):
        return 1

    # 步骤 3: 构建多平台 Go 程序
    ret = build_go_multiplatform(root_dir)

    if ret != 0:
        return ret

    print("\n" + "=" * 60)
    print("NPM 发布构建成功！")
    print("=" * 60)
    print("\n接下来可以运行:")
    print("  npm pack        # 生成 .tgz 包测试")
    print("  npm publish     # 发布到 npm")

    return 0


if __name__ == "__main__":
    sys.exit(main())
