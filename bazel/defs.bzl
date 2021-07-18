# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Bazel rules for running the xtoproto code generator."""
load("@io_bazel_rules_go//go:def.bzl", "go_library")


def _go_xtoproto_converter_impl(ctx):
    # The list of arguments we pass to the script.
    args = [
        "--alsologtostderr",
        "--codegen_request_json",
        struct(
            partial_request_path = ctx.files.request[0].path,
            converter_go_output = struct(
                short_path = ctx.outputs.out.short_path,
                path = ctx.outputs.out.path,
                root = ctx.outputs.out.root.path,
            ),
        ).to_json()
    ]

    # Action to run the Go code generator.
    ctx.actions.run(
        inputs = ctx.files.request,
        outputs = [ctx.outputs.out],
        arguments = args,
        progress_message = "Running xtoproto to generated %s" % ctx.outputs.out.short_path,
        executable = ctx.executable.xtoproto_tool,
    )

go_xtoproto_converter = rule(
    implementation = _go_xtoproto_converter_impl,
    attrs = {
        "request": attr.label(
            allow_single_file = [".pbtxt"],
            mandatory = True,
        ),
        "out": attr.output(mandatory = True),
        "xtoproto_tool": attr.label(
            executable = True,
            cfg = "host",
            allow_files = True,
            default = Label("//cmd/xtoproto"),
        ),
    },
)

def go_xtoproto_converter_library(name, request, importpath, deps=None, visibility=None, **kwargs):
    """Creates a go_library with the given name that will convert records to protobuf.
    
    Args:
        name: name of the go_library to generate.
        request:
        importpath:
        deps:
        visibility: visibility, passed to both go_library and other rules.
        **kwargs: Extra arguments that will be passed to go_library.
    """
    go_xtoproto_converter(
        name = name + "_converter",
        out = name + "_converter.go",
        request = request,
        visibility = visibility,
    )
    if deps == None:
        deps = []

    final_deps = deps + [
        "@xtoproto//csvtoprotoparse",
        "@xtoproto//protocp",
        "@xtoproto//csvcoder",
        "@xtoproto//textcoder",
        "@org_golang_google_protobuf//proto:go_default_library",
    ]
    go_library(
        name = name,
        srcs = [name + "_converter.go"],
        importpath = importpath,
        visibility = visibility,
        deps = final_deps,
        **kwargs,
    )
