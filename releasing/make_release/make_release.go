// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Program make_release helps make a release of xtoproto.
//
// It is currently only used to create a release for the gh-pages branch
// (website). With some modification it could also be used to tag the latest
// version of the repository.
//
// Example usage:
//
// 		bazel run //releasing/make_release -- --workspace $PWD --branch_suffix v006c --tag v0.0.6
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bmatcuk/doublestar"
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
	if *tag == "" {
		return fmt.Errorf("missing --tag flag")
	}
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
	releaseBranch := fmt.Sprintf("release%s", *releaseBranchSuffix)
	ghPagesBranch := fmt.Sprintf("gh-pages-release%s", *releaseBranchSuffix)
	if err := runCmd(exec.Command("git", "checkout", "-b", releaseBranch, "main")); err != nil {
		return fmt.Errorf("error creating release branch: %w", err)
	}
	// Generate the .pb.go files needed for the release.
	if err := runCmd(exec.Command("bazel", "run", "//releasing/generate_pb_go_files", "--", "-output_dir", filepath.Join(root, "proto"), "--alsologtostderr")); err != nil {
		return fmt.Errorf("error generating .pb.go files: %w", err)
	}
	pbgoFiles, err := doublestar.Glob("proto/**/*.pb.go")
	if err != nil {
		return fmt.Errorf("error matching .pb.go files: %w", err)
	}
	for _, f := range pbgoFiles {
		if err := runCmd(exec.Command("git", "add", "-f", f)); err != nil {
			return fmt.Errorf("error adding %q: %w", f, err)
		}
	}
	message := fmt.Sprintf("add .pb.go files for release %s", *tag)
	if err := runCmd(exec.Command("git", "commit", "-a", "-m", message)); err != nil {
		return fmt.Errorf("error committing .pb.go files: %w", err)
	}
	if err := runCmd(exec.Command("git", "tag", *tag)); err != nil {
		return fmt.Errorf("error tagging release: %w", err)
	}

	// Second, make the gh-pages release. This is done by running
	// `bazel run //cmd/xtoproto_web"` and outputting the static files to a
	// temporary directory, then copying those files back into a newly created
	// branch of the repo for the gh-pages release.
	if err := os.MkdirAll(*stagingDir, 0770); err != nil {
		return err
	}
	got, err := exec.Command("bazel", "run", "//cmd/xtoproto_web", "--", "--output_dir", *stagingDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error generating gh-pages content: %w/%s", err, string(got))
	}

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
	fmt.Printf("Created release branch %s; push it to the main repository with the command\n   git push google %s\n", releaseBranch, *tag)
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
