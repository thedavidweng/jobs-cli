# No local candidate profile store in v1

The CLI does not persist default candidate identity fields or resume paths in the profile directory. Every `apply prepare` receives explicit answers and file references from the user or Agent. Reuse of personal data is an Agent/workspace concern, not CLI state.
