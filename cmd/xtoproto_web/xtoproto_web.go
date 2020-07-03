package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/golang/glog"
)

var (
	addr                 = flag.String("addr", ":8081", "address to use for http serving")
	outputStaticFilesDir = flag.String("output_dir", "", "if non-empty, a copy of the static files will be output to this directory. This is used to generate the github pages release.")

	staticFiles = []staticFile{
		{"playground.wasm", "playground/playground.wasm"},
		{"index.html", "playground/index.html"},
		{"playground.html", "playground/index.html"},
		{"prism-theme-dark.css", "playground/prism-theme-dark.css"},
		{"prism-theme-light.css", "playground/prism-theme-light.css"},
		{"wasm_exec.js", "third_party/wasm_exec.js"},
	}
)

type staticFile struct {
	webPath, runfilesPath string
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	printRunfiles()
	fs := makeFilesys()
	if *outputStaticFilesDir != "" {
		if err := fs.outputGithubPagesFiles(*outputStaticFilesDir); err != nil {
			return fmt.Errorf("problem copying files to %s: %w", *outputStaticFilesDir, err)
		}
	}
	fmt.Printf("staring server at %s\n", *addr)
	return http.ListenAndServe(*addr, http.FileServer(fs))
}

func printRunfiles() error {
	rfe, err := bazel.ListRunfiles()
	if err != nil {
		return err
	}
	for _, e := range rfe {
		fi, err := os.Stat(e.Path)
		if err != nil {
			return err
		}

		glog.Infof("%q => %d: %q", e.ShortPath, fi.Size(), e.Path)
	}

	return nil
}

type filesys struct {
	webPathToWorkspacePath map[string]string
}

func makeFilesys() *filesys {
	m := make(map[string]string)
	for _, sf := range staticFiles {
		m[sf.webPath] = sf.runfilesPath
	}
	return &filesys{m}
}

func (fs *filesys) Open(name string) (http.File, error) {
	name = strings.TrimPrefix(name, "/")
	glog.Infof("requested %q", name)
	if name == "" {
		return &dir{fs: fs, webPath: ""}, nil
	}
	p, ok := fs.webPathToWorkspacePath[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	realPath, err := bazel.Runfile(p)
	if err != nil {
		return nil, err
	}
	return os.Open(realPath)
}

func (fs *filesys) outputGithubPagesFiles(dstDir string) error {
	for webPath, wsPath := range fs.webPathToWorkspacePath {
		realPath, err := bazel.Runfile(wsPath)
		if err != nil {
			return err
		}
		contents, err := ioutil.ReadFile(realPath)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(dstDir, webPath), contents, 0660); err != nil {
			return err
		}
	}
	return nil
}

type dir struct {
	fs      *filesys
	webPath string
}

func (d *dir) Close() error {
	return nil
}

func (d *dir) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("Read not supported on directory")
}

func (d *dir) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("Seek not supported on directory")
}

func (d *dir) Readdir(count int) ([]os.FileInfo, error) {
	var infos []os.FileInfo
	for webPath, wsPath := range d.fs.webPathToWorkspacePath {
		if !strings.HasPrefix(webPath, d.webPath) {
			continue
		}
		realPath, err := bazel.Runfile(wsPath)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(realPath)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (d *dir) Stat() (os.FileInfo, error) {
	wasmPath, err := bazel.Runfile(d.fs.webPathToWorkspacePath["playground.wasm"])
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(wasmPath)
	return os.Stat(dir)
}
