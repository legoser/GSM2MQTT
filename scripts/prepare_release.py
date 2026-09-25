#!/usr/bin/env python3
"""
prepare_release.py — Prepare a new project release.

Automates:
1. Determining next SemVer version (defaults to incrementing PATCH, e.g. v0.1.3 -> v0.1.4,
   allowing accumulating multiple micro-features into patch releases).
2. Selecting the correct previous tag strictly preceding the target release version.
3. Pre-flight safety checks (clean working tree, duplicate tag prevention).
4. Accumulating all commits since previous tag into CHANGELOG.md.
5. Creating atomic release commit and annotated git tag.
6. Outputting push instructions.
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


def parse_semver(tag):
    m = re.match(r"^v?(\d+)\.(\d+)\.(\d+)", tag)
    if m:
        return (int(m.group(1)), int(m.group(2)), int(m.group(3)))
    return (0, 0, 0)


def get_previous_tag(target_version=None):
    out = run_cmd(["git", "tag", "--list", "v*.*.*"], check=False)
    tags = [t.strip() for t in out.splitlines() if t.strip()]
    if not tags:
        return "v0.0.0"

    if target_version:
        target_semver = parse_semver(target_version)
        valid_tags = [t for t in tags if parse_semver(t) < target_semver]
        if valid_tags:
            valid_tags.sort(key=parse_semver, reverse=True)
            return valid_tags[0]

    tags.sort(key=parse_semver, reverse=True)
    return tags[0]


def bump_version(tag, bump_type="patch"):
    major, minor, patch = parse_semver(tag)
    if bump_type == "major":
        return f"v{major + 1}.0.0"
    elif bump_type == "minor":
        return f"v{major}.{minor + 1}.0"
    else:  # patch
        return f"v{major}.{minor}.{patch + 1}"


def is_working_tree_clean():
    out = run_cmd(["git", "status", "--porcelain"])
    return len(out.strip()) == 0


def tag_exists(tag):
    res = subprocess.run(
        ["git", "rev-parse", "--verify", "--quiet", f"refs/tags/{tag}"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return res.returncode == 0


def main():
    parser = argparse.ArgumentParser(description="Prepare a new release cut.")
    parser.add_argument("--version", "-v", default="", help="Explicit version (e.g. v0.1.4)")
    parser.add_argument("--bump", "-b", choices=["patch", "minor", "major"], default="patch",
                        help="Bump type if version not explicitly provided (default: patch)")
    parser.add_argument("--force", "-f", action="store_true", help="Overwrite existing local tag if present")
    parser.add_argument("--dry-run", action="store_true", help="Preview without making changes or tags")
    parser.add_argument("--from-tag", default="", help="Previous tag boundary (defaults to auto-detected previous tag)")
    args = parser.parse_args()

    # Determine target new tag
    if args.version:
        new_tag = args.version if args.version.startswith("v") else f"v{args.version}"
    else:
        latest = get_previous_tag()
        new_tag = bump_version(latest, args.bump)

    # Determine previous tag baseline strictly preceding new_tag
    previous_tag = args.from_tag or get_previous_tag(target_version=new_tag)

    print(f"==> Preparing Release: {new_tag} (previous baseline: {previous_tag})")

    # Safety checks
    if not args.dry_run and not is_working_tree_clean():
        print("Error: Working tree is not clean. Commit or stash changes first.", file=sys.stderr)
        sys.exit(1)

    if tag_exists(new_tag) and not args.force:
        sha = run_cmd(["git", "rev-parse", "--short", f"refs/tags/{new_tag}"])
        print(f"Error: Tag '{new_tag}' already exists pointing to commit {sha}.", file=sys.stderr)
        print(f"To remove stale local tag: git tag -d {new_tag}", file=sys.stderr)
        print(f"Or pass --force to overwrite: make prepare-release VERSION={new_tag} FORCE=1", file=sys.stderr)
        sys.exit(1)

    # Check commit difference
    commits = run_cmd(["git", "log", f"{previous_tag}..HEAD", "--oneline"], check=False)
    if not commits.strip():
        print(f"Warning: No commits found between {previous_tag} and HEAD.", file=sys.stderr)
        if not args.dry_run:
            sys.exit(1)

    # 1. Update changelog
    script_dir = os.path.dirname(os.path.abspath(__file__))
    gen_script = os.path.join(script_dir, "generate_release_notes.py")

    cmd = [sys.executable, gen_script, "--tag", new_tag, "--from-tag", previous_tag]
    if args.force:
        cmd.append("--force")

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
    staged_diff = run_cmd(["git", "diff", "--cached", "--name-only"], check=False)
    if staged_diff.strip():
        run_cmd(["git", "commit", "-m", commit_msg])
        print(f"✓ Created release commit: {commit_msg.splitlines()[0]}")
    else:
        print("Note: CHANGELOG.md was already up-to-date; skipping commit.")

    # 3. Create annotated tag
    tag_cmd = ["git", "tag", "-a", new_tag, "-m", f"Release {new_tag}"]
    if args.force:
        tag_cmd.insert(2, "-f")
    run_cmd(tag_cmd)
    print(f"✓ Created annotated tag {new_tag}")

    print("\n" + "=" * 60)
    print(f"🎉 Release {new_tag} is ready!")
    print("=" * 60)
    print("To publish this release to GitHub / Forgejo, run:")
    print(f"  git push origin main --tags\n")


if __name__ == "__main__":
    main()
