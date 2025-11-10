# Contributing to Zen Watcher

Thank you for your interest in contributing to Zen Watcher! We welcome contributions from the community.

## Code of Conduct

This project adheres to a code of conduct that we expect all contributors to follow. Please be respectful and constructive in all interactions.

## How to Contribute

### Reporting Bugs

If you find a bug, please open an issue with:
- A clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Your environment (Kubernetes version, Zen Watcher version, etc.)

### Suggesting Features

We welcome feature suggestions! Please open an issue with:
- A clear description of the feature
- Use cases and benefits
- Any implementation ideas you might have

### Submitting Pull Requests

1. **Fork the repository**
2. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes**:
   - Follow Go best practices and conventions
   - Add tests for new functionality
   - Update documentation as needed
   - Ensure all tests pass

4. **Commit your changes**:
   ```bash
   git commit -m "Add feature: description of your changes"
   ```

5. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Open a Pull Request**:
   - Provide a clear description of the changes
   - Reference any related issues
   - Ensure CI checks pass

## Development Setup

### Prerequisites

- Go 1.23 or later
- Kubernetes cluster (for testing)
- kubectl configured

### Building

```bash
cd src
go build -o zen-watcher .
```

### Running Tests

```bash
go test ./...
```

### Running Locally

```bash
export KUBECONFIG=~/.kube/config
export WATCH_NAMESPACE=zen-system
./zen-watcher
```

## Code Style

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions focused and concise

## Testing

- Write unit tests for new functionality
- Ensure existing tests pass
- Add integration tests where appropriate

## Documentation

- Update README.md for user-facing changes
- Add inline code comments for complex logic
- Update API documentation if CRD schemas change

## Review Process

1. All submissions require review
2. We aim to review PRs within 3-5 business days
3. Address review feedback promptly
4. Once approved, maintainers will merge your PR

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

## Questions?

Feel free to open an issue for any questions about contributing!


