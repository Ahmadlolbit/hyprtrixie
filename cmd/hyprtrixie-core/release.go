package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const releaseSigners = "manifests/release-signers"

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
	verify := []string{"-C", root(), "-c", "gpg.ssh.allowedSignersFile=" + signers, "-c", "gpg.openpgp.program=false", "-c", "gpg.x509.program=false", "verify-tag", tag}
	if run.Quiet(run.Quick, "git", verify...) != 0 {
		return false
	}
	object, _ := run.CaptureQuiet(run.Quick, "git", "-C", root(), "cat-file", "tag", tag)
	header, _, _ := strings.Cut(object, "\n\n")
	return slices.Contains(run.Lines(header), "tag "+tag)
}

func newestSigned(signers string) (string, []string) {
	var skipped []string
	for _, tag := range run.Lines(git("tag", "--list", "v*", "--sort=-v:refname")) {
		if signedByRelease(signers, tag) {
			return tag, skipped
		}
		skipped = append(skipped, tag)
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
	if run.Status("git", "-C", root(), "fetch", "--tags", "origin") != 0 {
		return "", "", errors.New("git fetch failed: no network, the clone has no origin, or a release tag was moved")
	}
	tag, skipped := newestSigned(signers)
	for _, name := range skipped {
		fmt.Printf("%s isn't signed with the release key, so it's left alone\n", name)
	}
	if tag == "" {
		return "", "", errors.New("no release tag is signed with the release key yet")
	}
	return tag, git("rev-parse", tag+"^{commit}"), nil
}

func afterUpdate(from, to string) int {
	dir := changesDir()
	text := changesBetween(from, to)
	if err := writeChanges(dir, text); err != nil {
		fmt.Fprintln(os.Stderr, "cannot save the update notes:", err)
	}
	fmt.Printf("\nWhat changed:\n%s\n", text)
	return run.Report(run.Exec("sudo", script(), "install", "--changes", filepath.Join(dir, "changes.txt")))
}

func updateHyprtrixie() int {
	fmt.Printf("Updating %s\n", root())
	tag, target, err := releaseTarget()
	if errors.Is(err, errNoReleaseKey) {
		return say("Hyprtrixie itself only updates to signed releases, and this copy has no release key yet, so it stays as it is.\n")
	}
	if err != nil {
		return fail("%v", err)
	}
	if run.Quiet(run.Quick, "git", "-C", root(), "merge-base", "--is-ancestor", target, "HEAD") == 0 {
		return say("Hyprtrixie is up to date (" + tag + ").\n")
	}
	from := git("rev-parse", "HEAD")
	if run.Status("git", "-C", root(), "merge", "--ff-only", target) != 0 {
		return fail("moving to %s failed: the folder has local changes, or its branch has gone its own way", tag)
	}
	return afterUpdate(from, git("rev-parse", "HEAD"))
}
