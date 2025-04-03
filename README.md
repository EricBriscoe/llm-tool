# llm-tool

A CLI tool for querying different LLM APIs and streaming responses.

## Installation

```bash
go build -o llm-tool ./cmd/llm-tool
```

## Configuration

The tool uses a configuration file located at `~/.config/llm-tool/config.yaml`. You can see the path with:

```bash
./llm-tool config path
```

Create a config file with the following structure:

```yaml
defaultProvider: openai
openai:
  apiKey: your_openai_api_key_here
  model: gpt-4o-mini
cboe:
  email: your_email@example.com
  token: your_cboe_token_here
  endpoint: http://ai.api.us.cboe.net:5005
  model: default
  datasource: my_custom_datasource  # Optional
gemini:
  apiKey: your_gemini_api_key_here
  model: gemini-2.0-flash-lite
```

## Usage

Query an LLM:

```bash
./llm-tool ask "What is the capital of France?"
```

Query Google Gemini:

```bash
./llm-tool ask --provider gemini "What is the capital of France?"
```

Review code changes between branches:

```bash
./llm-tool review main
```

Refactor files using an LLM:

```bash
# Single file refactoring
./llm-tool edit "Add error handling to all functions" myfile.go

# Multiple file refactoring
./llm-tool edit "Convert to using context throughout" file1.go file2.go file3.go 

# Process a file from stdin
cat myfile.go | ./llm-tool edit "Simplify the error handling logic"

# Output to different directory
./llm-tool edit "Convert to using generics" --output ./refactored/ myfile.go
```

## Options

- `--provider` (`-p`): LLM provider to use (openai, cboe, gemini) (defaults to config's defaultProvider)
- `--model` (`-m`): Model to use (defaults to provider's configured model)
- `--datasource` (`-d`): Data source to use with CBOE queries
- `--yes` (`-y`): Apply changes without confirmation (for edit command)
- `--output` (`-o`): Output directory for refactored files (for edit command)

## Supported Providers

- OpenAI
- Google Gemini
