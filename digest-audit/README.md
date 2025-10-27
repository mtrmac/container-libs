# Digest Audit Tool

A Go program that parses all Go code in a directory (recursively) and lists all uses of values with type `github.com/opencontainers/go-digest.Digest`.

## Features

- Finds all uses of `digest.Digest` values in Go code
- Outputs locations in VS Code-compatible format (`file:line:column`)
- Includes infrastructure for future filtering by use kind (variable, parameter, method call, comparison, etc.)

## Building

```bash
go build -o digest-audit
```

## Usage

```bash
# Analyze a directory
./digest-audit /path/to/directory

# Or run directly
go run main.go /path/to/directory
```

## Output Format

The output lists each use of a `digest.Digest` value in the format:

```
/path/to/file.go:line:column: identifierName
```

This format is compatible with VS Code and other editors that support file location navigation.

## Examples

Analyze the sample directory:
```bash
go run main.go sample
```

Analyze the storage module:
```bash
go run main.go ../storage
```

Analyze the entire repository:
```bash
go run main.go ..
```

## Testing

Run the tests with:
```bash
go test -v
```

The test suite includes:
- `TestSampleFile`: A rigid fixture-based test that validates the exact output (including line and column numbers) against `sample/expected_output.txt`

The test compares the actual output against a stored fixture to ensure any changes to the tool's behavior are caught immediately.

## Future Enhancements

The tool is designed to support filtering uses by kind:
- Variable declarations
- Assignments/copies
- Method call receivers
- Function parameters
- Comparisons
- Type conversions
- Return values

The `DigestUse.Kind` field is already populated with basic classification that can be extended as needed.

