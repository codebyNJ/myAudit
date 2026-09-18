package publish

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

type Result struct {
	PRURL  string `json:"pr_url"`
	Branch string `json:"branch"`
}

type Options struct {
	RunID      uuid.UUID
	NodeID     uuid.UUID
	RepoPath   string
	Git        *store.GitInfo
	SandboxDir string
	CommitSHA  string
	Title      string
}

func PublishFix(ctx context.Context, opts Options) (Result, error) {
	if opts.Git == nil || !opts.Git.HasGit || opts.Git.RemoteURL == "" {
		return Result{}, fmt.Errorf("source repo has no git remote — Push PR requires a cloned git repo with origin")
	}
	if opts.CommitSHA == "" {
		return Result{}, fmt.Errorf("no commit to publish for this ticket")
	}
	if err := preflightGH(ctx); err != nil {
		return Result{}, err
	}
	if dirty, err := repoDirty(ctx, opts.RepoPath); err != nil {
		return Result{}, err
	} else if dirty {
		return Result{}, fmt.Errorf("repo %s has uncommitted changes — commit or stash before Push PR", opts.RepoPath)
	}

	base := opts.Git.DefaultBranch
	if base == "" {
		base = "main"
	}
	branch := fmt.Sprintf("myaudit/%s/%s", shortID(opts.RunID), shortID(opts.NodeID))

	ws := sandbox.Workspace{Dir: opts.SandboxDir}
	patch, err := ws.FormatPatch(ctx, opts.CommitSHA)
	if err != nil {
		return Result{}, fmt.Errorf("format patch: %w", err)
	}

	// Everything below runs inside the developer's own clone, so whatever
	// branch they had checked out has to survive this — on every exit path.
	restore, err := checkoutWorkBranch(ctx, opts.RepoPath, branch, base)
	if err != nil {
		return Result{}, err
	}
	defer restore()

	amOut, amCode, amErr := gitAm(ctx, opts.RepoPath, patch)
	if amErr != nil {
		abortAm(ctx, opts.RepoPath)
		return Result{}, amErr
	}
	if amCode != 0 {
		// A failed `git am` leaves the repo mid-apply; without the abort the
		// restore checkout below cannot run and the clone stays wedged.
		abortAm(ctx, opts.RepoPath)
		restore()
		_, _, _ = sandbox.RepoGit(ctx, opts.RepoPath, "branch", "-D", branch)
		return Result{}, fmt.Errorf("git am failed — fix may conflict with current branch:\n%s", amOut)
	}

	pushOut, pushCode, pushErr := sandbox.RepoGit(ctx, opts.RepoPath, "push", "-u", "origin", branch)
	if pushErr != nil {
		return Result{}, pushErr
	}
	if pushCode != 0 {
		return Result{}, fmt.Errorf("git push: %s", pushOut)
	}

	title := "fix: " + strings.TrimSpace(opts.Title)
	if title == "fix: " {
		title = "fix: myAudit ticket"
	}
	body := fmt.Sprintf("Automated fix from myAudit run `%s`.\n\nCherry-picked commit `%s` from the audit sandbox.", opts.RunID, opts.CommitSHA[:min(12, len(opts.CommitSHA))])
	prOut, prCode, prErr := ghCreate(ctx, opts.RepoPath, title, body, branch, base)
	if prErr != nil {
		return Result{}, prErr
	}
	if prCode != 0 {
		return Result{}, fmt.Errorf("gh pr create: %s", prOut)
	}
	prURL := strings.TrimSpace(prOut)
	if prURL == "" {
		return Result{}, fmt.Errorf("gh pr create returned no URL")
	}
	return Result{PRURL: prURL, Branch: branch}, nil
}

// currentRef is the ref to come back to: the checked-out branch, or the raw
// commit when the repo is in detached HEAD.
func currentRef(ctx context.Context, dir string) string {
	if out, code, err := sandbox.RepoGit(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD"); err == nil && code == 0 {
		if ref := strings.TrimSpace(out); ref != "" {
			return ref
		}
	}
	out, code, err := sandbox.RepoGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

// checkoutWorkBranch puts dir on branch (forked from base) and returns a
// function that restores whatever was checked out beforehand. The returned
// func is safe to call more than once.
func checkoutWorkBranch(ctx context.Context, dir, branch, base string) (restore func(), err error) {
	orig := currentRef(ctx, dir)
	restore = func() {
		if orig == "" {
			return
		}
		_, _, _ = sandbox.RepoGit(ctx, dir, "checkout", orig)
	}

	_, _, _ = sandbox.RepoGit(ctx, dir, "fetch", "origin")
	out, code, err := sandbox.RepoGit(ctx, dir, "checkout", "-B", branch, "origin/"+base)
	if err != nil {
		return restore, err
	}
	if code != 0 {
		out2, code2, _ := sandbox.RepoGit(ctx, dir, "checkout", "-B", branch, base)
		if code2 != 0 {
			return restore, fmt.Errorf("checkout branch: %s %s", out, out2)
		}
	}
	return restore, nil
}

func abortAm(ctx context.Context, dir string) {
	_, _, _ = sandbox.RepoGit(ctx, dir, "am", "--abort")
}

func preflightGH(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found — install GitHub CLI and run `gh auth login`")
	}
	out, err := exec.CommandContext(ctx, "gh", "auth", "status").CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh not authenticated — run `gh auth login`: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func repoDirty(ctx context.Context, dir string) (bool, error) {
	out, code, err := sandbox.RepoGit(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, fmt.Errorf("git status: %s", out)
	}
	return strings.TrimSpace(out) != "", nil
}

func gitAm(ctx context.Context, dir, patch string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", "am")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)
	b, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(b), ee.ExitCode(), nil
		}
		return string(b), -1, err
	}
	return string(b), 0, nil
}

func ghCreate(ctx context.Context, dir, title, body, head, base string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--head", head, "--base", base, "--title", title, "--body", body)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	b, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(b), ee.ExitCode(), nil
		}
		return string(b), -1, err
	}
	return strings.TrimSpace(string(b)), 0, nil
}

func shortID(id uuid.UUID) string {
	s := id.String()
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}
