This is a CLI and Go client for the valuable data at pkg.go.dev, allowing you to programmatically find e.g. what code depends on your code, license information, etc. Since pkg.go.dev has no API, this scrapes the website to extract the data.

Some examples:

```
$ go build ./cmd/pkggodev

$ ./pkggodev package-info github.com/ipfs/go-ipfs | jq
{
  "Package": "github.com/ipfs/go-ipfs",
  "IsModule": true,
  "IsPackage": true,
  "Version": "v0.10.0",
  "Published": "2021-09-30",
  "License": "Apache-2.0, MIT, Apache-2.0, MIT",
  "HasValidGoModFile": true,
  "HasRedistributableLicense": true,
  "HasTaggedVersion": true,
  "HasStableVersion": false,
  "Repository": "github.com/ipfs/go-ipfs"
}

$ ./pkggodev imported-by github.com/ipfs/go-ipfs | jq -r .ImportedBy[] | head
gitee.com/Crazyrw/go-ipfs/cmd/ipfs
gitee.com/Crazyrw/go-ipfs/core
gitee.com/Crazyrw/go-ipfs/core/commands
gitee.com/Crazyrw/go-ipfs/core/corehttp
github.com/Angie3120/go-ipfs/cmd/ipfs
github.com/Angie3120/go-ipfs/core
github.com/Angie3120/go-ipfs/core/commands
github.com/Angie3120/go-ipfs/core/corehttp
github.com/BDWare/go-ipfs/cmd/ipfs
github.com/BDWare/go-ipfs/core

$ ./pkggodev search yaml | jq -r .Results[].Package | head -n 5
gopkg.in/yaml.v2
gopkg.in/yaml.v3
github.com/ghodss/yaml
sigs.k8s.io/yaml
sigs.k8s.io/kustomize/kyaml/yaml
```
