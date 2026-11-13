package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/deployed"
	"github.com/Ahmadlolbit/hyprtrixie/internal/lock"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	releaseSigners = "manifests/release-signers"
	writeAccess    = 2
)

var errNoReleaseKey = errors.New("no release key")

func trustedSigners() (string, func(), error) {
	keys, _ := run.CaptureQuiet(run.Quick, "git", "-C", root(), "show", "HEAD:"+releaseSigners)
	if strings.TrimSpace(keys) == "" {
		return "", func() {}, errNoReleaseKey
	}
	folder, err := os.MkdirTemp("", "hyprtrixie-signers-")
	if err != nil {
		return "", func() {}, err
	}
	done := func() { os.RemoveAll(folder) }
	path := filepath.Join(folder, "allowed_signers")
	if err := atomic.Write(path, []byte(keys+"\n"), 0o644); err != nil {
		done()
		return "", func() {}, err
	}
	return path, done, nil
}

func signedByRelease(signers, tag string) bool {
	verify := []string{"-C", root(), "-c", "gpg.ssh.allowedSignersFile=" + signers, "-c", "gpg.ssh.program=ssh-keygen", "-c", "gpg.openpgp.program=false", "-c", "gpg.x509.program=false", "verify-tag", tag}
	if run.Quiet(run.Quick, "git", verify...) != 0 {
		return false
	}
	object, _ := run.CaptureQuiet(run.Quick, "git", "-C", root(), "cat-file", "tag", tag)
	header, _, _ := strings.Cut(object, "\n\n")
	return slices.Contains(run.Lines(header), "tag "+tag)
}

func related(older, newer string) bool {
	return run.Quiet(run.Quick, "git", "-C", root(), "merge-base", "--is-ancestor", older, newer) == 0
}

func newestSigned(signers string) (string, []string) {
	var skipped []string
	for _, tag := range run.Lines(git("-c", "versionsort.suffix=-", "tag", "--list", "v*", "--sort=-v:refname")) {
		switch {
		case !related("HEAD", tag) && !related(tag, "HEAD"):
			skipped = append(skipped, tag+" is on a branch this copy doesn't follow, so it's left alone")
		case signedByRelease(signers, tag):
			return tag, skipped
		default:
			skipped = append(skipped, tag+" isn't signed with the release key, so it's left alone")
		}
	}
	return "", skipped
}

func releaseTarget() (string, string, error) {
	signers, done, err := trustedSigners()
	defer done()
	if err != nil {
		return "", "", err
	}
	if apt.Commands("ssh-keygen") != nil {
		return "", "", errors.New("checking a release's signature needs ssh-keygen: sudo apt install openssh-client")
	}
	if gitWrite("fetch", "--tags", "origin") != 0 {
		if !writable() {
			return "", "", errors.New("git fetch failed: " + root() + " is root's, so an account that can use sudo has to run hyprtrixie update, or there's no network")
		}
		return "", "", errors.New("git fetch failed: no network, the clone has no origin, or a release tag was moved")
	}
	tag, skipped := newestSigned(signers)
	for _, line := range skipped {
		fmt.Println(line)
	}
	if tag == "" {
		return "", "", errors.New("no release tag is signed with the release key yet")
	}
	return tag, git("rev-parse", tag+"^{commit}"), nil
}

func afterUpdate(from, to string) int {
	args := []string{script(), "install"}
	if text := changesBetween(from, to); text != "" {
		dir := changesDir()
		err := writeChanges(dir, text)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot save the update notes:", err)
		}
		fmt.Printf("\nWhat changed:\n%s\n", text)
		if err == nil {
			args = append(args, "--changes", filepath.Join(dir, "changes.txt"))
		}
	}
	return run.Report(run.Exec("sudo", args...))
}

func setupBehind(head string) (string, bool) {
	from := deployed.Finished()
	switch {
	case from == head:
		return "", false
	case from == "":
		from = deployed.LastCommit()
	}
	if from == "" {
		return "", false
	}
	if run.Quiet(run.Quick, "git", "-C", root(), "cat-file", "-e", from+"^{commit}") != 0 {
		from = head
	}
	return from, true
}

func finishSetup(upToDate string) int {
	head := git("rev-parse", "HEAD")
	from, behind := setupBehind(head)
	if !behind {
		return say(upToDate)
	}
	fmt.Println("The setup hasn't finished at this version yet, so it runs now.")
	if status := snapshotFirst(); status != 0 {
		return status
	}
	return afterUpdate(from, head)
}

func writable() bool {
	return syscall.Access(filepath.Join(root(), ".git"), writeAccess) == nil
}

func gitWrite(args ...string) int {
	command := append([]string{"git", "-C", root()}, args...)
	if !writable() {
		return run.Status("sudo", command...)
	}
	return run.Status(command[0], command[1:]...)
}

func gitPaths(args ...string) ([]string, bool) {
	output, err := run.CaptureQuiet(run.Slow, "git", append([]string{"-C", root()}, args...)...)
	if output == "" {
		return nil, err == nil
	}
	return strings.Split(strings.TrimSuffix(output, "\x00"), "\x00"), err == nil
}

func inMove(paths, moving []string) []string {
	var found []string
	for _, path := range paths {
		if slices.Contains(moving, path) {
			found = append(found, path)
		}
	}
	return found
}

func movedAlready(target string, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	differ, ok := gitPaths(append([]string{"diff", "--no-renames", "--name-only", "-z", target, "--"}, paths...)...)
	if !ok {
		return nil
	}
	var same []string
	for _, path := range paths {
		if !slices.Contains(differ, path) {
			same = append(same, path)
		}
	}
	return same
}

func addedAlready(target string, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	hashes, err := run.CaptureQuiet(run.Slow, "git", append([]string{"-C", root(), "hash-object", "--"}, paths...)...)
	listed, ok := gitPaths(append([]string{"ls-tree", "-z", target, "--"}, paths...)...)
	if err != nil || !ok {
		return nil
	}
	found := run.Lines(hashes)
	var same []string
	for index, path := range paths {
		if index < len(found) && slices.ContainsFunc(listed, func(line string) bool { return strings.HasSuffix(line, " blob "+found[index]+"\t"+path) }) {
			same = append(same, path)
		}
	}
	return same
}

func undoHalfMove(target string) {
	if run.Quiet(run.Quick, "git", "-C", root(), "merge-base", "--is-ancestor", "HEAD", target) != 0 {
		return
	}
	moving, ok := gitPaths("diff", "--no-renames", "--name-only", "-z", "HEAD", target)
	changed, okChanged := gitPaths("diff", "--no-renames", "--name-only", "-z", "HEAD")
	others, okOthers := gitPaths("ls-files", "-z", "--others", "--exclude-standard")
	if !ok || !okChanged || !okOthers {
		return
	}
	tracked, untracked := movedAlready(target, inMove(changed, moving)), addedAlready(target, inMove(others, moving))
	if len(tracked)+len(untracked) > 0 {
		fmt.Println("An earlier update stopped halfway, so the files it moved go back first.")
	}
	if len(tracked) > 0 {
		gitWrite(append([]string{"checkout", "-q", "HEAD", "--"}, tracked...)...)
	}
	if len(untracked) > 0 {
		gitWrite(append([]string{"clean", "-q", "-f", "--"}, untracked...)...)
	}
}

func updateHyprtrixie() int {
	fmt.Printf("Updating %s\n", root())
	tag, target, err := releaseTarget()
	if lock.Busy() {
		return fail("a Hyprtrixie run is going, so Hyprtrixie itself wasn't updated: run hyprtrixie update hyprtrixie once it ends")
	}
	switch {
	case errors.Is(err, errNoReleaseKey):
		fmt.Print("Hyprtrixie itself only updates to signed releases, and this copy has no release key yet, so it stays as it is.\n")
		return finishSetup("")
	case err != nil:
		return fail("%v", err)
	case run.Quiet(run.Quick, "git", "-C", root(), "merge-base", "--is-ancestor", target, "HEAD") == 0:
		return finishSetup("Hyprtrixie is up to date (" + tag + ").\n")
	}
	from := git("rev-parse", "HEAD")
	if status := snapshotFirst(); status != 0 {
		return status
	}
	undoHalfMove(target)
	if gitWrite("merge", "--ff-only", target) != 0 {
		return fail("moving to %s failed: the folder has local changes, or its branch has gone its own way", tag)
	}
	return afterUpdate(from, git("rev-parse", "HEAD"))
}
