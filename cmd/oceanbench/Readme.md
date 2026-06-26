# Ocean Bench

Ocean Bench, part of AusOcean's [Cloud Blue](https://www.cloudblue.org), is a cloud service for analyzing ocean data. It is written in Go and designed to run on Google App Engine Standard Edition.

## Installation and Usage

Before you begin, ensure you have **git**, **go**, and **npm** installed. For local development, it is also highly recommended to install [Air](https://github.com/air-verse/air) for live reloading.

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/ausocean/cloud.git
    cd cmd/oceanbench
    ```

2.  **Install dependencies:**
    ```bash
    npm install
    ```

3.  **Run a Local Development Instance**
    The easiest way to develop locally and test changes quickly is using Air. Air will monitor changes to `go`, `javascript`, `typescript`, and `html` files, and trigger a build, and browser refresh when connected to the proxied `:8090` port.
    ```bash
    air
    ```
    Once started, open your browser and navigate to `http://localhost:8090`.

4.  **Alternative without Air: Build the Project Manually**
    ```bash
    # 1. Compile TypeScript/Lit components and Tailwind CSS via Vite
    npm run build
    
    # 2. Compile the Go server
    go build
    
    # 3. Run the binary in standalone mode
    ./oceanbench --standalone --tvurl=http://localhost:8082 --cronurl=http://localhost:8081
    ```


### Command-Line Flags

The following flags are available when running the `oceanbench` binary, particularly in standalone mode:

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--standalone` | bool | `false` | Run in standalone mode without App Engine. |
| `--debug` | bool | `false` | Enable verbose output for debugging. |
| `--port` | int | `8080` | Port to listen on (can also be set via `PORT` env var). |
| `--host` | string | `localhost` | Hostname for the server. |
| `--filestore` | string | `store` | Path to the local file store. |
| `--testdata` | string | | Path to a JSON file to populate the standalone datastore. |
| `--loc` | string | | Latitude,longitude pair (e.g., `--loc -34.92,138.62`). |
| `--alt` | float | `0` | Altitude of the receiver (negative for depth). |
| `--gps` | string | | GPS receiver serial port (e.g., `/dev/ttyUSB0`). |
| `--baud` | int | `9600` | Baud rate for the GPS serial device. |
| `--cronurl` | string | | URL for the cron service. |
| `--tvurl` | string | | URL for the TV service. |

## Development

### Tailwind CSS
We use **Tailwind CSS v4** for styling Lit Elements. The main entry point is `ts/shared/tailwind.css`. Building of the production css file is handled by Vite during the build process.

### Project Structure
- `ts/`: TypeScript source files for Lit components.
- `s/`: Static files, including the generated `dist/` and `lit/` directories.
- `t/`: HTML templates used by the Go server.
- `*.go`: Go source files for the backend API and server.

## See Also
* [Ocean Bench service](https://bench.cloudblue.org)
* [AusOcean](https://www.ausocean.org)
