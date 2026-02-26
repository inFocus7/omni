# Dashie

Just a developer dashboard. Primarily made for myself, so I can track my contribution ratio (PR:Review ratio).

## GitHub Plugin

For the GitHub integration to work, you'll need to set the following environment variables:
- `GITHUB_TOKEN`
    - This is an access token used to authenticate with GitHub.
    - For more information read [GitHub's auth documentation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens).
    - My recommendation:
      - Classic Token
      - 30-day expiration (for security)
      - Scopes: **repo**, **read:user**
