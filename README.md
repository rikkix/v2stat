# v2stat

**v2stat** is a lightweight Go application designed to periodically record and store statistics from a V2Ray gRPC API into an SQLite database. It supports clean hourly logging and graceful shutdown.

## Features

- Connects to a V2Ray gRPC stats API
- Records statistics every hour
- Stores stats in a local SQLite database
- Gracefully handles shutdown on SIGINT/SIGTERM
- Configurable via command-line flags

## Requirements

- Go 1.18 or newer
- V2Ray with stats API enabled
- SQLite3
- gRPC client dependencies

## Installation

1. Clone the repository:
    ```bash
    git clone https://github.com/yourusername/v2stat.git
    cd v2stat
    ```

2. Build the project:
    ```bash
    go build -o v2stat
    ```

## Usage

```bash
v2stat --db path/to/v2stat.db --server 127.0.0.1:8080 --log-level info
```

## License

MIT