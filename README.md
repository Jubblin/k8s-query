# Kubernetes Cluster Data Collector

This tool collects information about your Kubernetes cluster and outputs it in both JSON and CSV formats. It can optionally push the data to a Git repository.

## Features

- Collects information about:
  - Namespaces
  - Pods
  - Containers
  - Environment Variables
  - Nodes
- Outputs data in both JSON and CSV formats
- Optional Git repository integration for data storage
- Timestamped output directories

## Prerequisites

- Go 1.16 or later
- Access to a Kubernetes cluster
- Git (if using Git repository integration)

## Installation

1. Clone this repository
2. Build the program:
   ```bash
   go build
   ```

## Configuration

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Edit the `.env` file with your configuration:
   ```bash
   # Git Repository Configuration
   K8S_GIT_REPO=github.com/username/repo.git
   K8S_GIT_USERNAME=your-username
   K8S_GIT_PASSWORD=your-password-or-token
   K8S_GIT_BRANCH=main

   # Optional: Custom kubeconfig location
   # KUBECONFIG=/path/to/your/kubeconfig
   ```

## Usage

1. Run the program:
   ```bash
   ./k8s_query
   ```

2. The program will:
   - Create a timestamped directory (e.g., `k8s-data-20240321-123456`)
   - Generate JSON and CSV files with cluster information
   - Push the data to Git if configured

## Output Files

The program generates the following files:

- `k8s_info.json`: Complete cluster information in JSON format
- `pods.csv`: Pod information
- `containers.csv`: Container information
- `env_vars.csv`: Environment variables
- `nodes.csv`: Node information

## Git Integration

To use Git integration:

1. Set up the required environment variables in your `.env` file:
   - `K8S_GIT_REPO`: Your Git repository URL
   - `K8S_GIT_USERNAME`: Your Git username
   - `K8S_GIT_PASSWORD`: Your Git password or token
   - `K8S_GIT_BRANCH`: Target branch (optional, defaults to 'main')

2. The program will automatically:
   - Initialize a Git repository in the output directory
   - Add all files
   - Commit with a timestamp
   - Push to the specified repository

## Security Notes

- Never commit your `.env` file to version control
- Use Git tokens instead of passwords when possible
- Keep your kubeconfig file secure

## License

MIT License # k8s-data
