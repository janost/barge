# bridge

A k9s-style terminal UI for AWS ECS and EC2. Navigate clusters, services, tasks, and instances with keyboard-driven drill-down, then exec right into containers or SSM sessions.

Built for cloud engineers, DevOps engineers, and SREs who are tired of juggling the AWS console and CLI to do simple operational tasks.

## Features

- **Interactive dashboard** &mdash; Browse ECS Clusters, Services, Tasks, Task Definitions, EC2 Instances, and Auto Scaling Groups in a single TUI
- **Hierarchical drill-down** &mdash; Navigate Cluster &rarr; Service &rarr; Task with Enter, or take alternate paths with secondary drill-down
- **ECS Exec & SSM Shell** &mdash; Open a shell in any ECS container or EC2 instance directly from the dashboard
- **Log tailing** &mdash; Stream CloudWatch logs for a service without hunting for log group names
- **Service health** &mdash; Quick status view: running/desired/pending tasks, deployments, recent stops
- **File copy** &mdash; Copy files to/from Fargate containers via `tar`-over-exec (no Docker socket needed)
- **Bookmarks & history** &mdash; Save frequent targets and reconnect in one command
- **Fuzzy search** &mdash; Filter any table view with substring matching across all columns
- **Configurable keybinds** &mdash; Customize shortcuts via YAML config
- **Bordered shell sessions** &mdash; PTY proxy with title bar showing context (cluster, service, instance)

## Install

### From source

```bash
go install github.com/janost/bridge@latest
```

### Build from repo

```bash
git clone https://github.com/janost/bridge.git
cd bridge
make build        # builds ./bridge
make build-all    # cross-compile to dist/
```

Requires Go 1.25+ and the AWS CLI v2 (for `ssm start-session` and `ecs execute-command`).

## Prerequisites

- **AWS CLI v2** installed and configured (`aws configure` or environment variables)
- **Session Manager Plugin** for SSM shell access ([install guide](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html))
- **ECS Exec enabled** on your ECS services for container exec ([enable guide](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html))

## Quick start

```bash
# Launch the interactive dashboard
bridge tui

# Or just
bridge
```

Use arrow keys or `j`/`k` to navigate, `Enter` to drill down or open actions, `Esc`/`Backspace` to go back, `/` to search, `q` to quit.

## Commands

| Command | Description |
|---------|-------------|
| `bridge tui` | Interactive dashboard (default) |
| `bridge exec` | Open shell in ECS task or EC2 instance |
| `bridge logs` | Tail CloudWatch logs for a service |
| `bridge status` | Service health summary |
| `bridge events` | Recent service event timeline |
| `bridge cp <src> <dst>` | Copy files to/from a container |
| `bridge list` | Tabular resource listing |
| `bridge exec save <name>` | Bookmark a connection target |
| `bridge exec connect <name>` | Reconnect via bookmark |
| `bridge exec bookmarks` | List/manage bookmarks |
| `bridge exec history` | Recent connection history |

### Global flags

```
-p, --profile   AWS profile name
-r, --region    AWS region
```

### Examples

```bash
# Exec into a specific service's most recent task
bridge exec -c my-cluster -s my-service

# Exec into an EC2 instance by ID
bridge exec -i i-0123456789abcdef0

# Tail logs for a service (last 5 minutes, follow)
bridge logs -c my-cluster -s my-service --since 5m

# Copy a file from a container
bridge cp -c my-cluster -s my-service :/var/log/app.log ./app.log

# Save a bookmark and reconnect later
bridge exec save prod-api -c production -s api-service
bridge exec connect prod-api
```

## Dashboard navigation

```
ResourcePicker
  ├── ECS Clusters
  │     ├── [Enter] Services in cluster
  │     │     └── [Enter] Tasks in service
  │     │           └── [Enter] ECS Exec into container
  │     └── [T] All tasks in cluster (secondary drill)
  ├── ECS Services (all clusters)
  │     └── [Enter] Tasks in service
  ├── ECS Tasks (all clusters)
  ├── ECS Task Definitions
  ├── EC2 Instances
  │     └── [Enter] SSM Shell
  └── Auto Scaling Groups
        └── [Enter] Member EC2 instances
```

## Configuration

Config file: `~/.config/bridge/config.yaml`

```yaml
keybinds:
  search: "/"
  quit: "q"
  refresh: "r"
  drill_alt: "T"
```

Bookmarks are stored in the same config file. History is stored in `~/.local/share/bridge/history.json`.

## License

MIT
