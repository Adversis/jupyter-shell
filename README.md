# jupyter-terminal

A little Go client that talks to Jupyter terminals over WebSocket. I wrote this because I needed to poke at Jupyter servers programmatically and got tired of using websocat with weird shell escaping.

## What's this do?

It connects to a Jupyter server and gives you a terminal session, just like clicking "New > Terminal" in the web UI. Except you're doing it from the command line, which is way cooler.

## Quick Start

First grab the dependencies:
```bash
go mod init jupyter-terminal
go get github.com/gorilla/websocket
go build -o jupyter-terminal main.go
```

Then connect to your Jupyter server:
```bash
./jupyter-terminal -url http://localhost:8888
```

That's it! You're now in a terminal on your Jupyter server.

## Real World Examples

**Local development server (no auth):**
```bash
./jupyter-terminal -url http://localhost:8888
```

**Production server with a token:**
```bash
./jupyter-terminal -url https://jupyter.company.com -token abc123def456
```

**Just run one command and bail:**
```bash
./jupyter-terminal -url http://localhost:8888 whoami
```

**Connect to a terminal you already created:**
```bash
./jupyter-terminal -url http://localhost:8888 -term 1
```

## How it actually works

Here's the deal - Jupyter terminals use a simple JSON protocol over WebSocket:
- You send: `["stdin", "ls -la\n"]` 
- You get back: `["stdout", "file1.txt\nfile2.txt\n"]`

This tool handles all that JSON nonsense for you. It also creates the terminal session using Jupyter's REST API before connecting to the WebSocket.

## The auth situation

Jupyter has a few auth modes:
- **No auth**: Common for local dev servers
- **Token auth**: You get a token from Jupyter and pass it with `-token`
- **Password auth**: Not supported (yet) - use tokens instead

## Gotchas

- The prompt detection is janky - we just print `$ ` and hope for the best
- If your server uses a self-signed cert, Go will complain (fix: use HTTP for local dev)
- Some commands that need a PTY might act weird

## Why not just use SSH?

Good question! Sometimes you can't:
- JupyterHub environments often don't give you SSH
- Container-based deployments might only expose HTTP
- You're testing Jupyter-specific security stuff
- You're already automating Jupyter and want to stay in that ecosystem

## Contributing

Found a bug? The WebSocket dies randomly? The JSON parsing explodes? PRs welcome!

## License

MIT - go wild
