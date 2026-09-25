#!/usr/bin/env python3
"""
prepare_release.py — Prepare a new project release.

Automates:
1. Determining next SemVer version (defaults to incrementing PATCH, e.g. v0.1.3 -> v0.1.4,
   allowing accumulating multiple micro-features into patch releases).
2. Accumulating all commits since previous tag into CHANGELOG.md.
3. Creating atomic release commit and annotated git tag.
4. Outputting push instructions.
"""

import argparse
import os
import re
import subprocess
import sys


def run_cmd(args, check=True):
    res = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    if check and res.returncode != 0:
        raise RuntimeError(f"Command failed: {' '.join(args)}\n{res.stderr}")
    return res.stdout.strip()


def get_latest_tag():
    out = run_cmd(["git", "tag", "--list", "v*.*.*", "--sort=-v:refname"], check=False)
    tags = [t.strip() for t in out.splitlines() if t.strip()]
    return tags[0] if tags else "v0.0.0"


def bump_version(tag, bump_type="patch"):
    m = re.match(r"^v?(\d+)\.(\d+)\.(\d+)$", tag)
    if not m:
        raise ValueError(f"Cannot parse SemVer tag: {tag}")
    major, minor, patch = int(m.group(1)), int(m.group(2)), int(m.group(3))
    if bump_type == "major":
        return f"v{major + 1}.0.0"
    elif bump_type == "minor":
        return f"v{major}.{minor + 1}.0"
    else:  # patch
        return f"v{major}.{minor}.{patch + 1}"


def is_working_tree_clean():
    out = run_cmd(["git", "status", "--porcelain"])
    return len(out.strip()) == 0


def main():
    parser = argparse.ArgumentParser(description="Prepare a new release cut.")
    parser.add_argument("--version", "-v", default="", help="Explicit version (e.g. v0.1.4)")
    parser.add_argument("--bump", "-b", choices=["patch", "minor", "major"], default="patch",
                        help="Bump type if version not explicitly provided (default: patch)")
    parser.add_argument("--dry-run", action="store_true", help="Preview without making changes or tags")
    parser.add_argument("--from-tag", default="", help="Previous tag boundary (defaults to latest tag)")
    args = parser.parse_args()

    latest_tag = args.from_tag or get_latest_tag()
    if args.version:
        new_tag = args.version if args.version.startswith("v") else f"v{args.version}"
    else:
        new_tag = bump_version(latest_tag, args.bump)

    print(f"==> Preparing Release: {new_tag} (previous tag: {latest_tag})")

    # Check commit difference
    commits = run_cmd(["git", "log", f"{latest_tag}..HEAD", "--oneline"], check=False)
    if not commits.strip():
        print(f"Warning: No commits found between {latest_tag} and HEAD.", file=sys.stderr)
        if not args.dry_run:
            sys.exit(1)

    if not args.dry_run and not is_working_tree_clean():
        print("Error: Working tree is not clean. Commit or stash changes first.", file=sys.stderr)
        sys.exit(1)

    # 1. Update changelog
    script_dir = os.path.dirname(os.path.abspath(__file__))
    gen_script = os.path.join(script_dir, "generate_release_notes.py")

    cmd = [sys.executable, gen_script, "--tag", new_tag, "--from-tag", latest_tag]
    if not args.dry_run:
        cmd.append("--update-changelog")
        run_cmd(cmd)
        print(f"✓ Updated CHANGELOG.md with [{new_tag.lstrip('v')}]")
    else:
        out = run_cmd(cmd)
        print("--- Dry-Run Changelog Preview ---")
        print(out)
        print("---------------------------------")
        return

    # 2. Commit release
    commit_msg = f"[chore] (release) Bump version to {new_tag}\n\n- Update CHANGELOG.md for {new_tag}"
    run_cmd(["git", "add", "CHANGELOG.md"])
    run_cmd(["git", "commit", "-m", commit_msg])
    print(f"✓ Created release commit: {commit_msg.splitlines()[0]}")

    # 3. Create annotated tag
    run_cmd(["git", "tag", "-a", new_tag, "-m", f"Release {new_tag}"])
    print(f"✓ Created annotated tag {new_tag}")

    print("\n" + "=" * 60)
    print(f"🎉 Release {new_tag} is ready!")
    print("=" * 60)
    print("To publish this release to GitHub / Forgejo, run:")
    print(f"  git push origin main --tags\n")


if __name__ == "__main__":
    main()
