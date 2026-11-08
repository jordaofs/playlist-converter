# Privacy

This describes Playlist Converter as deployed at https://playlist-converter.jfs-cloud.com, operated by Jordão Frazão. The served version is [`web/public/privacy.html`](../web/public/privacy.html); keep both in sync. If you run your own copy, publish your own operator, retention and contact details.

Playlist Converter stores a browser session identifier in an HttpOnly cookie. Session-owned records include provider account/channel identity, encrypted OAuth tokens, playlist metadata, source occurrences, match decisions and publication progress. Provider API requests disclose the necessary playlist/search information to Spotify and Google. The application does not download or upload audio.

There are no advertisements, analytics scripts or tracking cookies. The reverse proxy logs request time, path without query string, status, user agent and the client IP truncated to /24 (IPv4) or /48 (IPv6); it does not log cookies, referrers or authorization headers. Proxy logs are kept for up to 14 days and feed operator-only aggregate traffic reports. Application logs record conversion status codes and counts, and failed account connections as the service name and an error code, never playlist names, songs, links or account identities.

Sessions expire after seven days of inactivity; jobs expire after seven days. Cleanup runs periodically. Disconnect cancels dependent work and removes that provider connection. Clear local data removes the current principal's local records and credentials. Neither action deletes playlists already created remotely. Cookies cannot be recovered across devices or after deletion. Daily database backups stay on the same server and are deleted after seven days, so removed data can persist in a backup for up to seven more days.

Provider access remains subject to their policies and your consent. You can revoke access from the provider account settings.

Contact: [GitHub issues](https://github.com/jordaofs/playlist-converter/issues). Issues are public; do not include personal data, account details or tokens. Deleting your data is self-service (Clear my data, or revoke provider access). Report security vulnerabilities privately through [GitHub security advisories](https://github.com/jordaofs/playlist-converter/security/advisories/new) (see [SECURITY.md](../SECURITY.md)).
