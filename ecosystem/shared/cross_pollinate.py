"""
Astartes Ecosystem — Cross-Pollination Agent

ECOSYSTEM: cross-pollination

Reads CHANGELOG.md for unpropagated improvements and reports
which repos need updates. Designed to be run by a scheduled
Claude Code agent via /schedule.

Usage:
    python cross_pollinate.py           # Report pending propagations
    python cross_pollinate.py --check   # Exit 1 if pending propagations exist
"""

import re
import sys
from pathlib import Path
from dataclasses import dataclass


@dataclass
class PendingPropagation:
    """An improvement that needs to be propagated to other repos."""
    date: str
    origin_repo: str
    pattern_name: str
    description: str
    propagate_to: list[str]


def parse_changelog(changelog_path: str = None) -> list[PendingPropagation]:
    """Parse CHANGELOG.md for pending propagations."""
    if changelog_path is None:
        # Find changelog relative to this file
        ecosystem_dir = Path(__file__).parent.parent
        changelog_path = ecosystem_dir / "CHANGELOG.md"

    if not Path(changelog_path).exists():
        print(f"No changelog found at {changelog_path}")
        return []

    content = Path(changelog_path).read_text()
    pending = []

    # Parse entries
    # Format:
    # ## [date] [origin-repo] — [pattern-name]
    # **What changed:** ...
    # **Why:** ...
    # **Propagate to:** repo1, repo2
    # **Status:** pending | propagated

    entries = re.split(r'\n## ', content)
    for entry in entries[1:]:  # Skip header
        lines = entry.strip().split('\n')
        if not lines:
            continue

        # Parse header: date repo — pattern
        header_match = re.match(r'(\d{4}-\d{2}-\d{2})\s+(\S+)\s+[—-]\s+(.+)', lines[0])
        if not header_match:
            continue

        date_str = header_match.group(1)
        origin = header_match.group(2)
        pattern = header_match.group(3).strip()

        # Parse fields
        description = ""
        propagate_to = []
        status = "pending"

        for line in lines[1:]:
            line = line.strip()
            if line.startswith("**What changed:**"):
                description = line.replace("**What changed:**", "").strip()
            elif line.startswith("**Propagate to:**"):
                repos = line.replace("**Propagate to:**", "").strip()
                propagate_to = [r.strip() for r in repos.split(",")]
            elif line.startswith("**Status:**"):
                status = line.replace("**Status:**", "").strip().lower()

        if status == "pending" and propagate_to:
            pending.append(PendingPropagation(
                date=date_str,
                origin_repo=origin,
                pattern_name=pattern,
                description=description,
                propagate_to=propagate_to,
            ))

    return pending


def report_pending(pending: list[PendingPropagation]) -> str:
    """Generate a human-readable report of pending propagations."""
    if not pending:
        return "No pending propagations. Ecosystem is in sync."

    lines = [f"Found {len(pending)} pending propagation(s):\n"]
    for p in pending:
        targets = ", ".join(p.propagate_to)
        lines.append(f"  [{p.date}] {p.pattern_name}")
        lines.append(f"    Origin: {p.origin_repo}")
        lines.append(f"    Change: {p.description}")
        lines.append(f"    Needs propagation to: {targets}")
        lines.append("")

    return "\n".join(lines)


def main():
    check_mode = "--check" in sys.argv
    pending = parse_changelog()
    report = report_pending(pending)
    print(report)

    if check_mode and pending:
        sys.exit(1)


if __name__ == "__main__":
    main()
