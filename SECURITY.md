# Security

This experimental project has no production security assurance or supported-version policy yet.

Report vulnerabilities privately through [GitHub security advisories](https://github.com/jordaofs/playlist-converter/security/advisories/new). Do not open public issues for vulnerabilities, and never include credentials, tokens or user data in any report.

Keep `.env`, databases and encryption keys outside Git. Disconnect removes stored credentials; clearing local data leaves remotely created playlists intact. If a secret is exposed, revoke/rotate it with the affected provider and review repository history before further publication.

Deployment credentials for the maintainer's instance live only in the protected `production` environment, released after a manual approval, and the server restricts the deploy key to a single command. Pull requests and forks never receive them.
