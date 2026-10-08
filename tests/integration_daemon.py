"""Exercise the real daemon with a private config and a throwaway bare remote."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
import threading
import time
import unittest


def git(repo, *args, check=True):
    return subprocess.run(["git", "-C", str(repo), *args], text=True,
                          capture_output=True, check=check, timeout=15)


class DaemonTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.remote = self.root / "remote.git"
        self.repo = self.root / "repo"
        self.repo.mkdir()
        git(self.root, "init", "--bare", str(self.remote))
        git(self.repo, "init", "-b", "main")
        git(self.repo, "config", "user.name", "Sync test")
        git(self.repo, "config", "user.email", "sync-test@example.invalid")
        git(self.repo, "config", "auto-sync.syncInterval", "300")
        (self.repo / ".gitignore").write_text(".beads/embeddeddolt/\n")
        (self.repo / ".beads").mkdir()
        (self.repo / ".beads/issues.jsonl").write_text('{"id":"fixture"}\n')
        (self.repo / "note.md").write_text("before")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-m", "fixture")
        git(self.repo, "remote", "add", "origin", str(self.remote))
        git(self.repo, "push", "-u", "origin", "main")
        self.runtime = self.repo / ".beads/embeddeddolt/obsidian/.dolt/noms"
        self.runtime.mkdir(parents=True)
        self.runtime.chmod(0)
        self.config = self.root / "config/git-auto-sync"
        self.config.mkdir(parents=True)
        self.env = dict(os.environ, HOME=str(self.root), XDG_CONFIG_HOME=str(self.root / "config"))
        for key in ("NOTIFY_SOCKET", "INVOCATION_ID", "JOURNAL_STREAM"):
            self.env.pop(key, None)
        self.process = None
        self.log = (self.root / "daemon.log").open("w+")

    def tearDown(self):
        if self.process and self.process.poll() is None:
            self.process.terminate()
            try: self.process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=5)
        self.runtime.chmod(0o700)
        self.log.seek(0)
        print(self.log.read(), end="")
        self.log.close()
        self.temp.cleanup()

    def start(self, repo):
        (self.config / "config.json").write_text(json.dumps({"repos": [str(repo)]}))
        self.process = subprocess.Popen([DAEMON], env=self.env, stdout=self.log, stderr=self.log)

    def wait(self, predicate):
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            self.assertIsNone(self.process.poll(), "daemon exited unexpectedly")
            if predicate(): return
            time.sleep(0.1)
        self.fail("daemon did not sync the expected change")

    def remote_has(self, path, expected):
        result = git(self.remote, "show", "refs/heads/main:" + path, check=False)
        return result.returncode == 0 and result.stdout == expected

    def test_event_sync_with_ignored_runtime_churn_and_new_directory(self):
        self.start(self.repo)
        def watching():
            fds = Path("/proc") / str(self.process.pid) / "fd"
            try: return any(os.readlink(fd) == "anon_inode:inotify" for fd in fds.iterdir())
            except OSError: return False
        self.wait(watching)
        self.runtime.chmod(0o700)
        stop = threading.Event()
        def churn():
            i = 0
            while not stop.is_set():
                path = self.runtime / ("nbs_manifest_" + str(i))
                path.write_text("temporary runtime data")
                path.unlink()
                i += 1
                stop.wait(0.01)
        thread = threading.Thread(target=churn)
        thread.start()
        try:
            (self.repo / "note.md").write_text("after")
            self.wait(lambda: self.remote_has("note.md", "after"))
            (self.repo / ".beads/issues.jsonl").write_text('{"id":"fixture","status":"closed"}\n')
            self.wait(lambda: self.remote_has(".beads/issues.jsonl", '{"id":"fixture","status":"closed"}\n'))
            (self.repo / "new empty directory").mkdir()
            time.sleep(1)
            (self.repo / "new empty directory/child.md").write_text("child note")
            self.wait(lambda: self.remote_has("new empty directory/child.md", "child note"))
            names = git(self.remote, "ls-tree", "-r", "--name-only", "refs/heads/main").stdout
            self.assertNotIn("embeddeddolt", names)
            self.assertIsNone(self.process.poll())
        finally:
            stop.set()
            thread.join(timeout=5)

    def test_repository_start_failure_exits_process(self):
        self.start(self.root / "missing repository")
        self.assertNotEqual(self.process.wait(timeout=5), 0)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--daemon", required=True)
    args = parser.parse_args()
    DAEMON = str(Path(args.daemon).resolve())
    unittest.main(argv=[__file__], verbosity=2)
