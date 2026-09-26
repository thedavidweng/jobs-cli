# Resolver redirect policy

Application URL resolution follows at most five redirects. Cross-host redirects are allowed (short tracking links are common). Clear-text HTTPS→HTTP downgrades and targets in private/link-local/metadata address ranges are rejected. The resolver remains an HTTP client with a bounded policy, not a browser.
