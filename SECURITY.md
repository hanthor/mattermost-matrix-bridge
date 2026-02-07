# Security Policy

## Supported Versions

This bridge is currently in **pre-release** status. Security updates will be provided for:

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please follow these steps:

### 🔒 For Security Issues (Confidential)

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please report security issues via:

1. **Email**: [security contact - to be configured]
2. **GitHub Security Advisory**: Use the "Security" tab → "Report a vulnerability" (if enabled)

### 📧 What to Include

Please include as much information as possible:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if you have one)
- Your name/handle for acknowledgment (optional)

### ⏱️ Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Timeline**: Depends on severity
  - Critical: 1-2 weeks
  - High: 2-4 weeks
  - Medium: 1-2 months
  - Low: Best effort

### 🎖️ Recognition

We maintain a security acknowledgments file for researchers who responsibly disclose vulnerabilities. If you'd like to be credited, please let us know.

## Known Security Issues

See [SECURITY_REVIEW.md](./SECURITY_REVIEW.md) for a comprehensive security analysis.

### Current Critical Issues (As of 2026-02-07)

⚠️ **This bridge is NOT production-ready due to the following issues:**

1. **Weak Password Generation** - Auto-generated Matrix passwords are predictable
2. **Optional Authentication** - Slash commands may accept requests without authentication
3. **Plaintext Credentials** - User tokens stored unencrypted in database

**Status**: These issues are documented in SECURITY_REVIEW.md with fixes provided in SECURITY_FIXES.md

### We Are Working On

- Implementing cryptographically secure password generation
- Enforcing mandatory authentication for all endpoints
- Adding at-rest encryption for stored credentials
- Implementing rate limiting
- Comprehensive input validation

## Security Best Practices for Deployment

Until critical issues are resolved, we recommend:

### For Development/Testing Only

- ✅ Use isolated test networks
- ✅ Don't store sensitive data
- ✅ Use firewall rules to restrict access
- ✅ Regular security updates
- ✅ Monitor logs for suspicious activity

### NOT Recommended for Production

- ❌ Public-facing deployments
- ❌ Production data
- ❌ Untrusted networks
- ❌ Compliance-sensitive environments

### When Deploying (Post-fixes)

1. **Strong Tokens**
   ```bash
   # Generate secure tokens
   openssl rand -base64 32
   ```

2. **Encryption Key**
   ```bash
   # Generate and store securely
   export BRIDGE_ENCRYPTION_KEY=$(openssl rand -base64 32)
   ```

3. **Network Security**
   - Use TLS/SSL for all connections
   - Restrict access with firewall rules
   - Use reverse proxy (nginx/caddy)
   - Enable fail2ban or similar

4. **Monitoring**
   - Set up log aggregation
   - Monitor for authentication failures
   - Alert on unusual patterns
   - Regular security audits

5. **Updates**
   - Subscribe to security advisories
   - Test updates in staging
   - Maintain backup/rollback procedures

## Security Features

### Planned Features

- [x] Security review completed
- [ ] Credential encryption at rest
- [ ] Rate limiting on all endpoints
- [ ] Comprehensive input validation
- [ ] Audit logging
- [ ] Security headers
- [ ] Token rotation support
- [ ] Two-factor authentication support
- [ ] End-to-bridge encryption (E2B)

## Security Contact

- **Project Maintainer**: [To be configured]
- **Security Team**: [To be configured]
- **PGP Key**: [To be configured]

## Disclosure Policy

We follow a **coordinated disclosure** policy:

1. **Report received** → Acknowledged within 48 hours
2. **Investigation** → Severity assessment and fix development
3. **Fix ready** → Security advisory prepared
4. **Coordination** → 90-day disclosure window for affected parties
5. **Public disclosure** → Advisory published with fix available

### Exceptions

We may expedite public disclosure for:
- Actively exploited vulnerabilities
- Vulnerabilities already publicly known
- Critical infrastructure risks

## Bug Bounty

We currently do not offer a bug bounty program. However, we deeply appreciate responsible disclosure and will:

- Credit you in our security acknowledgments
- Mention you in release notes (if desired)
- Send you bridge swag (if available)

## Third-Party Dependencies

We rely on several third-party libraries. Security issues in dependencies are tracked separately:

- **mautrix-go**: https://github.com/mautrix/go
- **Mattermost SDK**: https://github.com/mattermost/mattermost/server
- Others: See [go.mod](./go.mod)

For dependency vulnerabilities:
1. Update to patched version
2. Release security update
3. Notify users

## Security Advisories

Security advisories will be published:

- GitHub Security Advisories
- SECURITY_ADVISORIES.md (in this repo)
- Release notes
- Project documentation

## Compliance

This bridge is designed to be compatible with:

- **GDPR**: Data minimization, user data deletion
- **SOC 2**: Audit logging, access controls (when implemented)
- **HIPAA**: Encryption requirements (when implemented)

Specific compliance guidance will be provided in separate documentation.

## Security Audit History

| Date | Auditor | Scope | Status |
|------|---------|-------|--------|
| 2026-02-07 | Internal | Comprehensive code review | ✅ Complete |
| TBD | External | Third-party audit | 📅 Planned |

## Additional Resources

- [SECURITY_REVIEW.md](./SECURITY_REVIEW.md) - Detailed security analysis
- [SECURITY_FIXES.md](./SECURITY_FIXES.md) - Implementation guide
- [SECURITY_SUMMARY.md](./SECURITY_SUMMARY.md) - Quick reference
- [Matrix Security Guidelines](https://matrix.org/docs/guides/security-disclosure-policy)

## Questions?

For non-security questions:
- Open a GitHub issue
- Check existing documentation
- Join the community chat (TBD)

---

**Thank you for helping keep this project secure!**

Last Updated: 2026-02-07
