#!/usr/bin/env python
import pprint
import subprocess
import sys
from typing import List


def usage():
    usage_text = """Usage: {0} <commit-range> [<path> ...]

Warning: This tool relies on dot and feh to generate and open the colored graph.

Parameters:
    * <commit-range> is either "unstaged" or <first-commit>[:<last-commit>]
      with <commit-range> defaulting to "unstaged" and <last-commit> defaulting
      to HEAD. When "unstaged" is specified, the tool will comapre the unstaged
      version with the last commit.
    * <path> can be specified multiple times but this will open as many times feh.
    * When <path> is not specified, the tool finds all the JSON dumps updated 
      in the commit range.""".format(sys.argv[0])

    print(usage_text, file=sys.stderr)


def find_updated_dumps(first: str, last: str):
    cmd = "git diff --name-only {first} "
    if last != "unstaged":
        cmd += "{last} "
    
    cmd = (cmd + "| egrep '*.json'").format(first=first, last=last)

    res = subprocess.run(cmd, shell=True, check=True, stdout=subprocess.PIPE, text=True)
    dumps = str(res.stdout).split("\n")

    return [dump.strip() for dump in dumps if dump != ""]


def diff_path(first: str, last: str, path: str):
    args = {"tool": "./tools/diff-dotgraph.py",
            "first": first,
            "last": last,
            "path": path.strip()}
    
    cmd = """{tool} "$(git show {first}:{path} | zbuild llbgraph)" """.format(**args)
    if last != "unstaged":
        cmd += "\"$(git show {last}:{path} | zbuild llbgraph --raw)\""
    else:
        cmd += "\"$(cat {path} | zbuild llbgraph --raw)\" "
    
    cmd += "| dot /dev/stdin -o /dev/stdout -T png | feh -"
    cmd = cmd.format(**args)
    subprocess.run(cmd, shell=True, check=True)


def diff_commit_range(commit_range: str, paths: List[str]):
    commits = commit_range.split(":")
    if len(commits) > 2:
        print("ERROR: Invalid commit range.\n", file=sys.stderr)
        usage()
        sys.exit(1)
    if commits[0] == "unstaged":
        commits = ["HEAD", "unstaged"]
    if len(commits) == 1:
        commits.append("HEAD")

    if len(paths) == 0:
        paths = find_updated_dumps(commits[0], commits[1])

    for path in paths:
        print("Generting diff for {0}...".format(path))
        diff_path(commits[0], commits[1], path)


if __name__ == "__main__":
    if len(sys.argv) == 2 and sys.argv[1] == "--help":
        usage()
        sys.exit(0)

    commit_range = sys.argv[1] if len(sys.argv) >= 2 else "unstaged"
    paths = sys.argv[2:] if len(sys.argv) >= 3 else []
    
    diff_commit_range(commit_range, paths)
