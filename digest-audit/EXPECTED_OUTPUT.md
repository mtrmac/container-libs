# Expected Output for Sample File

This document shows the expected output when running the digest-audit tool on the sample directory.

## Sample File Contents

The `sample/sample.go` file demonstrates various uses of `digest.Digest`:
- Variable declarations
- Assignments/copies
- Method calls
- Comparisons
- Type conversions
- Function parameters
- Return values

## Expected Output

When running:
```bash
go run main.go sample
```

Expected output (line numbers may vary if sample.go is modified):
```
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:11:16: Digest
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:11:6: d1
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:12:2: d2
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:15:2: d3
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:15:8: d1
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:18:10: d1
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:22:11: d2
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:22:5: d1
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:27:29: d3
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:31:16: d1
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:34:2: d4
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:35:6: d4
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:38:20: d
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:38:29: Digest
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:40:13: d
/Users/mitr/Go/src/container-libs/digest-audit/sample/sample.go:44:25: Digest
```

## Explanation of Results

Each line shows:
- **File path**: Absolute path to the source file
- **Line number**: Line where the digest.Digest use occurs
- **Column number**: Column where the identifier starts
- **Identifier name**: The name of the variable/parameter/type being used

Note that the tool finds:
- Type names in declarations (e.g., `Digest` in `var d1 digest.Digest`)
- Variable names (e.g., `d1`, `d2`, `d3`, `d4`)
- Parameter names (e.g., `d` in function `processDigest`)
- All uses of these identifiers throughout the code

## Real-World Usage

The tool successfully processes large codebases. For example, running on the image module:
```bash
./digest-audit ../image | wc -l
# Output: 2234
```

This found 2,234 uses of digest.Digest values across the image module.

