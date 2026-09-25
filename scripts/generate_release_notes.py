#!/usr/bin/env python3
"""
generate_release_notes.py — Automated release notes and changelog generator.

Parses Git log between tags using Conventional Commits and commit body
descriptions, formatting them into Keep a Changelog Markdown.

Can be run:
  1. Standalone / CI: to produce release_notes.md for GitHub/Forgejo releases.
  2. With --update-changelog: to automatically record changes into CHANGELOG.md.
"""

import argparse
import datetime
import os
import re
import subprocess
import sys

CATEGORY_MAP = [
    ("feat", "Features & Improvements"),
    ("fix", "Bug Fixes"),
    ("perf", "Performance"),
    ("refactor", "Refactoring"),
    ("docs", "Documentation"),
    ("build", "Build & Packaging"),
    ("ci", "CI/CD & Automation"),
    ("test", "Tests"),
    ("chore", "Maintenance"),
]

DEFAULT_CATEGORY = "Other Changes"


def run_git(args, check=True):
    res = subprocess.run(
        ["git"] + args,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if check and res.returncode != 0:
        raise RuntimeError(f"Git command failed: git {' '.join(args)}\n{res.stderr}")
    return res.stdout.strip()


def get_previous_tag(current_tag=None):
    tags_output = run_git(["tag", "--list", "v*.*.*", "--sort=-v:refname"], check=False)
    tags = [t.strip() for t in tags_output.splitlines() if t.strip()]
    if current_tag and current_tag in tags:
        tags.remove(current_tag)
    return tags[0] if tags else None


def get_commits(range_spec):
    cmd = ["log", range_spec, "--no-merges", "--format=COMMIT_SEP%n%H%n%s%n%b"]
    output = run_git(cmd, check=False)
    if not output:
        return []

    raw_commits = output.split("COMMIT_SEP\n")
    commits = []
    for raw in raw_commits:
        if not raw.strip():
            continue
        lines = raw.strip().splitlines()
        if len(lines) < 2:
            continue
        h = lines[0][:7]
        subj = lines[1].strip()
        body_lines = [line.rstrip() for line in lines[2:] if line.strip()]
        commits.append((h, subj, body_lines))
    return commits


def parse_conventional_commit(subj):
    m = re.match(r"^([a-zA-Z]+)(?:\(([^)]+)\))?!?:\s*(.+)$", subj)
    if m:
        ctype = m.group(1).lower()
        scope = m.group(2).strip()
        desc = m.group(3).strip()
        return ctype, scope, desc
    return "other", None, subj


def group_commits(commits):
    cat_lookup = dict(CATEGORY_MAP)
    grouped = {}

    for h, subj, body in commits:
        ctype, scope, desc = parse_conventional_commit(subj)
        cat = cat_lookup.get(ctype, DEFAULT_CATEGORY)
        grouped.setdefault(cat, []).append((h, scope, desc, body))

    # Maintain defined category order
    ordered = {}
    for _, cat_title in CATEGORY_MAP:
        if cat_title in grouped:
            ordered[cat_title] = grouped[cat_title]
    if DEFAULT_CATEGORY in grouped:
        ordered[DEFAULT_CATEGORY] = grouped[DEFAULT_CATEGORY]

    return ordered


def format_markdown(grouped, tag_name, range_spec):
    lines = []
    lines.append(f"## What's changed in {tag_name} ({range_spec})\n")

    if not grouped:
        lines.append("No changes recorded in this release.\n")
    else:
        for cat_title, items in grouped.items():
            lines.append(f"### {cat_title}\n")
            for h, scope, desc, body in items:
                prefix = f"**{scope}**: " if scope else ""
                lines.append(f"- {prefix}{desc} (`{h}`)")
                for b in body:
                    b_strip = b.strip()
                    if b_strip.startswith(("-", "*")):
                        lines.append(f"  {b_strip}")
                    else:
                        lines.append(f"    {b_strip}")
            lines.append("")

    # Install instructions section
    lines.append("## Installation\n")
    lines.append("### Standalone Binaries")
    lines.append("```bash")
    lines.append("sha256sum -c checksums.txt")
    lines.append(f"tar -xzf gsm2mqtt-{tag_name}-linux-amd64.tar.gz")
    lines.append("./gsm2mqtt --version")
    lines.append("```\n")

    lines.append("### OpenWrt Packages")
    lines.append("```bash")
    lines.append("# OpenWrt <= 23.05 (OPKG):")
    lines.append(f"opkg install gsm2mqtt_{tag_name.lstrip('v')}-1_x86_64.ipk")
    lines.append("")
    lines.append("# OpenWrt >= 25.12 (APK):")
    lines.append(f"apk add --allow-untrusted ./gsm2mqtt-{tag_name.lstrip('v')}-r1-x86_64.apk")
    lines.append("")
    lines.append("/etc/init.d/gsm2mqtt start")
    lines.append("```")

    return "\n".join(lines).strip() + "\n"


def update_changelog_file(changelog_path, tag_name, grouped):
    if not os.path.exists(changelog_path):
        print(f"Warning: {changelog_path} not found, skipping update.", file=sys.stderr)
        return

    with open(changelog_path, "r", encoding="utf-8") as f:
        content = f.read()

    clean_ver = tag_name.lstrip("v")
    header_pattern = f"## [{clean_ver}]"
    if header_pattern in content:
        print(f"Note: {changelog_path} already contains section {header_pattern}. Skipping update.")
        return

    today = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d")
    section_lines = [f"## [{clean_ver}] - {today}\n"]

    for cat_title, items in grouped.items():
        section_lines.append(f"### {cat_title}\n")
        for h, scope, desc, body in items:
            prefix = f"**{scope}**: " if scope else ""
            section_lines.append(f"- {prefix}{desc} (`{h}`)")
            for b in body:
                b_strip = b.strip()
                if b_strip.startswith(("-", "*")):
                    section_lines.append(f"  {b_strip}")
                else:
                    section_lines.append(f"    {b_strip}")
        section_lines.append("")

    new_section = "\n".join(section_lines)

    # Insert under ## [Unreleased]
    unreleased_idx = content.find("## [Unreleased]")
    if unreleased_idx != -1:
        insert_pos = content.find("\n", unreleased_idx)
        if insert_pos != -1:
            updated = content[:insert_pos + 1] + "\n" + new_section + content[insert_pos + 1:]
        else:
            updated = content + "\n\n" + new_section
    else:
        updated = content + "\n\n" + new_section

    with open(changelog_path, "w", encoding="utf-8") as f:
        f.write(updated)
    print(f"Successfully updated {changelog_path} with {header_pattern}")


def git_ref_exists(ref):
    if not ref:
        return False
    res = subprocess.run(
        ["git", "rev-parse", "--verify", "--quiet", ref],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return res.returncode == 0


def main():
    parser = argparse.ArgumentParser(description="Automate changelog and release notes from git commits.")
    parser.add_argument("--tag", default=os.getenv("TAG", ""), help="Current release tag (e.g., v0.2.0)")
    parser.add_argument("--from-tag", default="", help="Previous tag (defaults to auto-detected previous tag)")
    parser.add_argument("--output", "-o", default="", help="File to write release notes to (e.g., release_notes.md)")
    parser.add_argument("--update-changelog", action="store_true", help="Automatically update CHANGELOG.md")
    parser.add_argument("--changelog-file", default="CHANGELOG.md", help="Path to CHANGELOG.md")
    args = parser.parse_args()

    tag = args.tag
    if not tag:
        tag = run_git(["describe", "--tags", "--always"], check=False) or "dev"
    if not tag.startswith("v") and re.match(r"^[0-9]", tag):
        tag = "v" + tag

    from_tag = args.from_tag
    if not from_tag:
        from_tag = get_previous_tag(current_tag=tag)

    if from_tag:
        if git_ref_exists(tag) and tag != "dev":
            range_spec = f"{from_tag}..{tag}"
        else:
            range_spec = f"{from_tag}..HEAD"
    else:
        if git_ref_exists(tag) and tag != "dev":
            range_spec = tag
        else:
            range_spec = "HEAD"

    commits = get_commits(range_spec)
    grouped = group_commits(commits)
    notes_markdown = format_markdown(grouped, tag, range_spec)

    if args.output:
        with open(args.output, "w", encoding="utf-8") as f:
            f.write(notes_markdown)
        print(f"Generated release notes in {args.output}")
    else:
        print(notes_markdown)

    if args.update_changelog:
        update_changelog_file(args.changelog_file, tag, grouped)


if __name__ == "__main__":
    main()
