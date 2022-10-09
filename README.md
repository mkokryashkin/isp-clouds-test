# Test task for ISPRAS Cloud Department: Simple Golang Server
## Docs
API specification can be found [here](https://app.swaggerhub.com/apis-docs/fckxorg/isp-clouds-test/1.0.0).

## Requirements
- First of all, you need to [install](https://go.dev/doc/install) the Golang suite.
- Then, you need to make sure you have the [ffmpeg](https://ffmpeg.org/download.html) installed. It is unlikely that it is not present on your PC already, if you are running any popular Linux distro.

## Building
```
go build -v ./...
```

## Testing
```
go test -v ./...
```
