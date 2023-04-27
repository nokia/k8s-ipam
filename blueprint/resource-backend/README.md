# resource-backend

## Description
resource-backend controller

## Usage

### Fetch the package
`kpt pkg get REPO_URI[.git]/PKG_PATH[@VERSION] resource-backend`
Details: https://kpt.dev/reference/cli/pkg/get/

### View package content
`kpt pkg tree resource-backend`
Details: https://kpt.dev/reference/cli/pkg/tree/

### Apply the package
```
kpt live init resource-backend
kpt live apply resource-backend --reconcile-timeout=2m --output=table
```
Details: https://kpt.dev/reference/cli/live/
