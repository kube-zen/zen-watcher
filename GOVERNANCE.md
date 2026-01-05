# Governance

This document describes the governance model for the Zen Watcher project.

---

## Overview

Zen Watcher is an open-source project maintained by the Kube-ZEN team. We follow a **benevolent dictator** model with clear maintainer responsibilities and community-driven decision-making.

---

## Maintainers

### Current Maintainers

**Zen Team** - zen@kube-zen.io

Maintainers are responsible for:
- Reviewing and merging pull requests
- Triaging issues and security reports
- Maintaining code quality and project standards
- Releasing new versions
- Ensuring security and compliance
- Setting project direction and priorities
- Resolving conflicts and enforcing code of conduct

### Becoming a Maintainer

Maintainers are selected based on:
- Consistent, high-quality contributions
- Deep understanding of the project and its goals
- Ability to review code and provide constructive feedback
- Commitment to the project's long-term success
- Alignment with project values and principles

**Process**: Maintainers are invited by existing maintainers after demonstrating sustained contribution and alignment with project goals.

---

## Decision-Making Process

### Technical Decisions

**Small Changes** (bug fixes, documentation, minor features):
- Maintainer review and approval
- No formal process required

**Medium Changes** (new features, API changes, architectural improvements):
- Discussion in GitHub Issues or Discussions
- Maintainer review and approval
- May require design document for complex changes

**Large Changes** (breaking changes, major architectural shifts):
- Design document or RFC (Request for Comments)
- Community discussion period (minimum 1 week)
- Maintainer consensus required
- Documented in design docs or architecture documentation

### Release Decisions

- **Patch Releases** (1.2.0 → 1.2.1): Maintainer decision
- **Minor Releases** (1.2.0 → 1.3.0): Maintainer consensus
- **Major Releases** (1.2.0 → 2.0.0): Maintainer consensus + community notification

### Conflict Resolution

- Technical disagreements: Discussion in GitHub Issues/Discussions
- Code review conflicts: Maintainer final decision
- Code of conduct violations: See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

---

## Project Structure

### Repositories

- **zen-watcher** (this repository): Core application code
- **helm-charts**: Helm chart repository (separate repository)
- **zen-sdk**: Shared SDK components (separate repository)

### Versioning

- Follows [Semantic Versioning](https://semver.org/)
- Version defined in `VERSION` file (single source of truth)
- All components synchronized (image, chart, code, git tag)
- See [docs/VERSIONING.md](docs/VERSIONING.md) for details

---

## Contribution Process

### How to Contribute

1. **Fork the repository**
2. **Create a feature branch** from `main`
3. **Make your changes** following [CONTRIBUTING.md](CONTRIBUTING.md)
4. **Write tests** for new features
5. **Update documentation** as needed
6. **Submit a pull request** with clear description

### Pull Request Process

1. **Open PR**: Create pull request with clear description
2. **CI Checks**: All CI checks must pass
3. **Code Review**: At least one maintainer approval required
4. **Discussion**: Address review comments
5. **Merge**: Maintainer merges after approval

### Review Criteria

- Code quality and style
- Test coverage
- Documentation updates
- Backward compatibility
- Security implications
- Performance impact

---

## Communication Channels

### Primary Channels

- **GitHub Issues**: Bug reports, feature requests, questions
- **GitHub Discussions**: General discussion, Q&A
- **Email**: zen@kube-zen.io (general inquiries)
- **Security Email**: security@kube-zen.io (vulnerability reports)

### Response Times

- **Security Issues**: 24 hours (see [SECURITY.md](SECURITY.md))
- **Bug Reports**: 48 hours (acknowledgment)
- **Feature Requests**: 1 week (initial response)
- **General Questions**: Best effort

---

## Code of Conduct

All contributors and maintainers must follow the [Code of Conduct](CODE_OF_CONDUCT.md).

**Enforcement**: Maintainers are responsible for enforcing the code of conduct. Violations can be reported to zen@kube-zen.io.

---

## Project Principles

1. **Security First**: Security is a top priority for a security tool
2. **Kubernetes-Native**: Leverage Kubernetes primitives and patterns
3. **Pure Core**: Core stays focused (no egress, no secrets, no external dependencies)
4. **Extensibility**: Ecosystem components handle integrations
5. **Documentation**: Comprehensive, accurate, and up-to-date
6. **Backward Compatibility**: Maintain compatibility within major versions
7. **Community-Driven**: Open to community contributions and feedback

---

## Release Process

### Release Schedule

- **No Fixed Schedule**: Releases happen when ready
- **Patch Releases**: As needed for bug fixes and security patches
- **Minor Releases**: When new features are ready
- **Major Releases**: For breaking changes (rare)

### Release Process

1. **Version Update**: Update `VERSION` file
2. **Changelog**: Update `CHANGELOG.md` with release notes
3. **Release Notes**: Create GitHub release notes (see `RELEASE_NOTES_v1.2.0.md` template)
4. **Testing**: Full test suite passes
5. **Build**: Build and push Docker image
6. **Helm Chart**: Update and publish Helm chart
7. **Git Tag**: Create git tag (v-prefixed)
8. **GitHub Release**: Create GitHub release with notes
9. **Documentation**: Update documentation if needed

See [docs/VERSIONING.md](docs/VERSIONING.md) for detailed versioning strategy.

---

## Roadmap

The project roadmap is maintained in [ROADMAP.md](ROADMAP.md).

**Roadmap Process:**
- Maintainers set high-level direction
- Community input via GitHub Issues/Discussions
- Roadmap updated quarterly or as priorities change

---

## License

Zen Watcher is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

---

## Contact

**General Inquiries**: zen@kube-zen.io  
**Security Issues**: security@kube-zen.io  
**GitHub Issues**: https://github.com/kube-zen/zen-watcher/issues  
**Documentation**: https://github.com/kube-zen/zen-watcher/tree/main/docs

---

**Last Updated**: 2025-01-05

