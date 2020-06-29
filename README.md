# xtoproto (csvtoproto, xmltoproto, etc.)

xtoproto is a library for automatically (1) inferring a protocol buffer
definition (a `.proto` file) from XML and CSV files, and (2) generating
runtime code from translating XML and CSV files to proto using the mappings
from (1).

## Quickstart {#quickstart}

### Stage 1 {#quickstart-stage-1}

```shell
infer_csv_to_proto_mapping -- \
  --csv <path to CSV file that fits in memory> \
  --mapping_out <path to .pbtxt output> \
  --package <package for the generated .proto file> \
  --message <short name for the generated message>
```

### Stage 2 {#quickstart-stage-2}

```shell
csv_to_proto_codegen -- \
  --mapping_pbtxt <path to .pbtxt output from stage 1> \
  --proto_out <path to output .proto file> \
  --go_out <path to output .go file with generaed translation code>
```

## Disclaimer

This is not an official Google product.
