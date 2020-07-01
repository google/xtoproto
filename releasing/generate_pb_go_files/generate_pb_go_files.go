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

// Program generate_pb_go_files collects the generated go files from the bazel
// runfiles directory that match a given prefix and outputs those files to a
// destination directory; this may be used when .pb.go artifacts are needed to
// build xtoproto without bazel.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/golang/glog"
	"golang.org/x/sync/errgroup"
)

const (
	outDirMode  os.FileMode = 0770
	outFileMode os.FileMode = 0660
)

var (
	importPath = flag.String("import_path", "github.com/google/xtoproto/proto", "import path used for generated proto files")
	outputDir  = flag.String("output_dir", "", "output directory")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	rfe, err := bazel.ListRunfiles()
	if err != nil {
		return err
	}

	cfg := &config{
		ImportPath: *importPath,
		OutputDir:  *outputDir,
	}
	eg := &errgroup.Group{}
	for _, e := range rfe {
		e := e
		eg.Go(func() error {
			return cfg.maybeCopy(e)
		})
	}
	return eg.Wait()
}

type config struct {
	ImportPath string `json:"import_path"`
	OutputDir  string `json:"output_dir"`
}

func (c *config) destination(e bazel.RunfileEntry) string {
	idx := strings.Index(e.ShortPath, c.ImportPath+"/")
	if idx == -1 || !strings.HasSuffix(e.ShortPath, ".go") {
		return ""
	}
	rest := e.ShortPath[idx+len(c.ImportPath)+1:]
	return filepath.Join(c.OutputDir, rest)
}

func (c *config) maybeCopy(e bazel.RunfileEntry) error {
	dst := c.destination(e)
	if dst == "" {
		return nil
	}
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, outDirMode); err != nil {
		return err
	}
	contents, err := ioutil.ReadFile(e.Path)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(dst, contents, outFileMode)
	if err != nil {
		return err
	}
	glog.Infof("finished copying %s", dst)
	return nil
}
