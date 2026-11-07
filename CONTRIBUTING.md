# Contributing

Run the synthetic local setup and checks in README.md before proposing changes. Add a failing behavioral regression test before changing matching, authorization or publication behavior. Preserve order and duplicate occurrences, keep provider differences in adapters, and do not silently retry unknown remote mutations.

Use synthetic fixtures in tests and screenshots. Never include provider credentials, databases, private playlists or user tokens in an issue or pull request. Live-provider mutation tests are opt-in and require the account owner's authorization.

Use conventional commit subjects (`fix:`, `feat:`, `docs:`, `test:`). Describe the resulting behavior, relevant trade-offs and validation. Keep changes focused. CI runs offline provider tests and browser journeys without production credentials.
