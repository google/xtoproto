# xtoproto (csvtoproto, xmltoproto, etc.)

xtoproto is a library for automatically (1) inferring a protocol buffer
definition (a `.proto` file) from XML and CSV files, and (2) generating
runtime code from translating XML and CSV files to proto using the mappings
from (1).

## Not yet operable

This repository was created on June 29, 2020. We are incrementally migrating the
code onto Github, and the project will not be functional until that migration is
finished. This README will be updated with instructions about how to use the
project once the migration is complete. In the mean time, feel free to browse
the code.

## Building

The project is buildable with Bazel. We also plan to make it buildable using `go
build` after the initial migration effort.

```
bazel build //...
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

## Disclaimer

This is not an official Google product.
