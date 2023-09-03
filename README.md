LiteFS Cloud Go Client
======================

This project is a thin wrapper for connecting to LiteFS Cloud. Most methods
require either an org-scoped or cluster-scoped auth token. The API on this
package is not considered stable until it reaches v1.0.0.

## Usage

To use the library, instantiate a `Client` and set the auth token if your
method requires it:

```go
client := lfsc.NewClient()
client.Token = os.Getenv("LITEFS_CLOUD_TOKEN")
```
