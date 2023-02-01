# xtoproto (csvtoproto, xmltoproto, etc.)

xtoproto is a library for automatically (1) inferring a protocol buffer
definition (a `.proto` file) from XML and CSV files, and (2) generating
runtime code that translates XML and CSV files to proto using the mappings
from (1).

## Quickstart

The best way to get started is to try out xtoproto using the [interactive,
web-based playground hosted on Github](https://google.github.io/xtoproto).

![screenshot of xtoproto playground](https://raw.githubusercontent.com/google/xtoproto/gh-pages/images/playground-example.png
"xtoproto playground")


More details about how to use xtoproto will be added soon.

## Design

xtoproto uses example data to infer a both *protobuf definition*, a *source
record spec*, and a *mapping definition*.

* The *protobuf definition* can be
represented in a `.proto` file with a custom `message` type for the record being
inferred.
* The *source record spec* is a specification for well-formed input data. For a
  CSV-to-proto translation, this would be a schema for the CSV input: names and
  types of columns.
* The *mapping definition* defines how to convert from the source record spec to
  the destination proto type.


### Formats
The following formats currently have some support:

1. CSV (comma separated values).

## Building

The project is buildable with Bazel and `go build`. Bazel is recommended because
the files needed for `go build` are only present in the release branches of the
repository (`v0.0.6`, `v0.0.5`, etc.).

```
bazel build //...
```

## Playground

Try out xtoproto using the [interactive, web-based playground hosted on
Github](https://google.github.io/xtoproto). The playground uses a
WebAssembly version of xtoproto and does not transmit the input example data to
a remote server. Alternatively, you may start the playground on your workstation
with this command, then navigate to http://localhost:8888/

```shell
bazel run //cmd/xtoproto_web -- --addr ":8888"
```

## Development

`gopls` [does not yet work with
bazel](https://github.com/golang/go/issues/37205). In the meantime, it is
convenient to generate the `.pb.go` files used within this project so that gopls
can pick them up and make autocomplete work. To do this, issue the following
command from the root of the checked out xtoproto repository:


```shell
 bazel run //releasing/generate_pb_go_files -- -output_dir $PWD/proto --alsologtostderr
```

### Releasing

There is a script for generating the release. Run it from the a cloned
repository with the following command.

```shell
git remote add google  git@github.com:google/xtoproto.git
bazel run //releasing/make_release -- --workspace $PWD --branch_suffix v006c --tag v0.0.6
```

## Disclaimer

This is not an official Google product.


This repository was created on June 29, 2020. We are incrementally migrating the
code onto Github, and the project will not be functional until that migration is
finished. This README will be updated with instructions about how to use the
project once the migration is complete. In the mean time, feel free to browse
the code.
