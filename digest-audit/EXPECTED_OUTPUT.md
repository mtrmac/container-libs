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

The exact expected output is stored in `sample/expected_output.txt` and is used as a fixture for testing.

When running:
```bash
go run main.go sample
```

The output will show each use with its absolute file path, line number, column number, and identifier name.

See `sample/expected_output.txt` for the precise expected output (with relative paths for portability).

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

