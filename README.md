# Vanity Sniper
Listens for vanity changes on configured alt accounts and attempts to snipe them to configured servers.

# Building
- Clone the project
- Build a binary using `go build -ldflags "-s -w"`

# Running
- Rename `sample_config.json` to `config.json`.
- Place the `config.json` file in the same directory as the built executable.
- Run the binary when you are done modifying the configuration.

# Prerequisite(s)
- [Go](https://go.dev/doc/install) (**Only for building**)
