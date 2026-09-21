# Setup

## Self-Hosted Runner

- Get the `APP_ID` and `APP_PRIVATE_KEY` by registering an app through the org on GitHub
    - Go to org settings > developer settings > GitHub apps
    - New GitHub app
    - Give it a name and website, disable webhooks, and enable the organization permission "Self-hosted runners" as read/write
    - Copy the app id and download a private key
    - Go to org settings > actions > runner groups
    - Create a runner group and restrict access to certain repos then allow access to public repos
- To put the private key as a line in the env file, replace all the line breaks with `\n`

```ini
# Fill
APP_ID=
APP_PRIVATE_KEY=

# As-Is
APP_LOGIN=Best-Capstone-Team-org
RUNNER_SCOPE=org
ORG_NAME=Best-Capstone-Team-org
RUNNER_GROUP=Infrastructure
RUNNER_WORKDIR=/tmp/runner
LABELS=self-hosted
```

## Pre-Commit Hook

Formatting and linting run on staged files before each commit, matching what CI checks. Run `git config core.hooksPath .githooks` to enable it and bypass it with `git commit --no-verify`.
