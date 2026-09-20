# Contributing to qemu-driver

## Developer Certificate of Origin

Contributions must be signed off. By adding a `Signed-off-by` line to your
commit you certify the [Developer Certificate of Origin 1.1](DCO): that you
wrote the contribution or otherwise have the right to submit it under the
project's license.

Sign off with git's built-in flag:

```
git commit -s
```

which appends a line of the form

```
Signed-off-by: Your Name <your@email.example>
```

Commits without a sign-off will be rejected in review.

## Before you push

```
just ci
```

runs the same checks CI does: lint and formatting, build, tests, and a
vulnerability scan. `just fmt` applies the formatters.

## Licensing of contributions

qemu-driver is AGPL-3.0-or-later. Every contribution is licensed under
AGPL-3.0-or-later.
