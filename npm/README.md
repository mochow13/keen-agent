# Keen Agent

Keen Agent is a terminal-based agent runner for working on local codebases.

## Install with script

```bash
curl -fsSL https://raw.githubusercontent.com/mochow13/keen-agent/main/scripts/install.sh | bash
```

To pin a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/mochow13/keen-agent/main/scripts/install.sh | bash -s -- -v v0.1.4
```

Installs to `/usr/local/bin` if writable, otherwise `$HOME/.local/bin`.

## Install with `npm`

Install the CLI globally:

```bash
npm install -g keen-agent
```

Update the global install:

```bash
npm install -g keen-agent@latest
# or
npm update -g keen-agent
```

`npm update` without `-g` only updates local project dependencies.

Check that the install worked:

```bash
keen-agent --version
which keen-agent
```

You can also run it without a global install:

```bash
npx keen-agent --version
```

## Run Keen Agent

Start Keen Agent in your current directory:

```bash
keen-agent
```
