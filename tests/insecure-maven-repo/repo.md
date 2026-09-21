# insecure-maven-repo

Regression test for **konveyor/analyzer-lsp#1190** (fixed by **#1191**): MTA's
`mvn.insecure.enabled` toggle was a no-op because the wagon-only insecure flag is ignored by
Maven 3.9+'s native resolver transport, so analysis against a Maven repository with an
untrusted TLS certificate silently produced empty results.

## What it exercises

- **Sample app:** `https://github.com/dymurray/mta-insecure-maven-test` — a minimal Maven
  project with a single dependency (`javax.servlet:javax.servlet-api:4.0.1`) and one servlet
  using the `javax.servlet` API.
- **Insecure Maven repo:** an in-cluster nginx reverse-proxy to Maven Central served over a
  **self-signed** TLS certificate (`local-maven-resources/insecure-mirror/`). All Maven
  traffic is mirrored to it (`mirrorOf: *`).
- **Hub setting:** `mvn.insecure.enabled = true`.

When the dependency resolves (insecure mode honored), JDTLS indexes the project and the
`jakarta-ee` target flags the `javax.servlet` usage. When it does not (the bug), resolution
fails PKIX, the project imports empty, and no incidents are produced — so this test's
exact-match validation fails.

## Running it

This test is not part of the default hub run. It is driven by the dedicated
`maven-insecure-*` Makefile targets and the `hub-insecure-maven` CI workflow, which stand up
the mirror, enable the hub setting, and run only this test. Point the java provider image at a
build that contains the #1190 fix to see it pass:

```
make hub-install JAVA_PROVIDER_IMG=<java-provider image with the #1190 fix>
make maven-insecure-mirror maven-insecure-enable
make test-hub-insecure
```
