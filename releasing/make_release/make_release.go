// Program make_release helps make a release of xtoproto.
//
// It is currently only used to create a release for the gh-pages branch
// (website). With some modification it could also be used to tag the latest
// version of the repository.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/golang/glog"
)

var (
	projectDir          = flag.String("workspace", "", "path to workspace directory")
	stagingDir          = flag.String("staging", "/tmp/xtoproto-staging", "path to staging directory")
	releaseBranchSuffix = flag.String("branch_suffix", "", "suffix for git branches created during the release process")
	tag                 = flag.String("tag", "", "should be something like v0.0.5")
	repo                = flag.String("google_xproto_repository", "https://github.com/google/xtoproto.git", "the main repository to use for generating the release")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root := *projectDir
	if root == "" {
		r, err := os.Getwd()
		if err != nil {
			return nil
		}
		root = r
	}
	if err := os.Chdir(root); err != nil {
		return err
	}
	glog.Infof("running commands from %s", root)
	if err := runCmd(exec.Command("git", "diff-index", "--quiet", "HEAD")); err != nil {
		return fmt.Errorf("git diff-index --quiet HEAD detected differences; ensure git repo is clean: %v", err)
	}
	if err := os.MkdirAll(*stagingDir, 0770); err != nil {
		return err
	}

	got, err := exec.Command("bazel", "run", "//cmd/xtoproto_web", "--", "--output_dir", *stagingDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error generating gh-pages content: %w/%s", err, string(got))
	}
	ghPagesBranch := fmt.Sprintf("gh-pages-release%s", *releaseBranchSuffix)
	if err := runCmd(exec.Command("git", "fetch", *repo, fmt.Sprintf("gh-pages:%s", ghPagesBranch))); err != nil {
		return fmt.Errorf("failed to create gh pages branch: %w", err)
	}
	if err := runCmd(exec.Command("git", "checkout", ghPagesBranch)); err != nil {
		return fmt.Errorf("failed to checkout gh pages branch: %w", err)
	}
	files, err := filepath.Glob(filepath.Join(*stagingDir, "*"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := runCmd(exec.Command("cp", "-R", f, root+"/")); err != nil {
			return fmt.Errorf("failed to copy %q to %q: %w", f, *stagingDir, err)
		}
	}
	fmt.Printf("Generated gh-pages content into new gh-paged-dervied branch %s.\nInsepct the output and push the release with\n\n  git push google %s:gh-pages\n", ghPagesBranch, ghPagesBranch)
	return nil
}

func runCmd(c *exec.Cmd) error {
	glog.Infof("issuing command: %s", c)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w / %s", err, string(out))
	}
	return nil
}
