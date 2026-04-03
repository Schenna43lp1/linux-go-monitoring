# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest (`main`) | ✅ |
| older releases | ❌ |

Only the latest version on the `main` branch receives security fixes.

---

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report security issues privately via **[GitHub Security Advisories](../../security/advisories/new)** or by emailing the maintainer directly (see profile).

### What to include

- A clear description of the vulnerability
- Steps to reproduce / proof of concept
- Potential impact (e.g. privilege escalation, data exposure)
- Your suggested fix, if any

### Response timeline

| Step | Target |
|------|--------|
| Acknowledgement | within 48 hours |
| Status update | within 7 days |
| Fix / patch release | within 30 days (severity-dependent) |

---

## Security Considerations

**linux-monitor** reads system metrics (CPU, RAM, disk, network, sensors) using [`gopsutil`](https://github.com/shirou/gopsutil) and displays them via a local GUI. It does **not**:

- open any network ports
- store or transmit data remotely
- require elevated privileges at runtime

The install script (`instrall.sh`) uses `sudo` to copy the binary to `/usr/local/bin` and create a `.desktop` entry — review it before running.

---

## Dependency Security

Dependencies are managed via Go modules. To audit for known vulnerabilities:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```
