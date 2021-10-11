This is a CLI and Go client for the valuable data at pkg.go.dev, allowing you to programmatically find e.g. what code depends on your code, license information, etc. Since pkg.go.dev has no API, this scrapes the website to extract the data.

View package metadata:
```
$ go build ./cmd/pkggodev

$ ./pkggodev package-info github.com/ipfs/go-ipfs
Package:                        github.com/ipfs/go-ipfs
IsModule:                       true
IsPackage:                      true
Version:                        v0.10.0
Published:                      2021-09-30
License:                        Apache-2.0, MIT, Apache-2.0, MIT
HasValidGoModFile:              true
HasRedistributableLicense:      true
HasTaggedVersion:               true
HasStableVersion:               false
Repository:                     github.com/ipfs/go-ipfs
```

Every command supports json output:
```
$ ./pkggodev package-info github.com/ipfs/go-ipfs --format json | jq
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
```

Find packages that import a package:
```
$ ./pkggodev imported-by github.com/ipfs/go-ipfs | head
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
```

Search for packages:
```
$ ./pkggodev search yaml --limit 2
Package:        gopkg.in/yaml.v2
Version:        v2.4.0
Published:      2020-11-17
ImportedBy:     25575
License:        Apache-2.0
Synopsis:       Package yaml implements YAML support for the Go language.

Package:        gopkg.in/yaml.v3
Version:        v3.0.0-...-496545a
Published:      2021-01-07
ImportedBy:     2634
License:        Apache-2.0, MIT
Synopsis:       Package yaml implements YAML support for the Go language.

```
