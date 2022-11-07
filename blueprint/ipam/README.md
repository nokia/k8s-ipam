# ipam

## Description
ipam controller

## Usage

### Fetch the package
`kpt pkg get REPO_URI[.git]/PKG_PATH[@VERSION] ipam`
Details: https://kpt.dev/reference/cli/pkg/get/

### View package content
`kpt pkg tree ipam`
Details: https://kpt.dev/reference/cli/pkg/tree/

### Apply the package
```
kpt live init ipam
kpt live apply ipam --reconcile-timeout=2m --output=table
```
Details: https://kpt.dev/reference/cli/live/
