# Test task for ISPRAS Cloud Department: Simple Golang Server
![GitHub Workflow Status](https://img.shields.io/github/workflow/status/fckxorg/isp-clouds-test/Go)
![GitHub](https://img.shields.io/github/license/fckxorg/isp-clouds-test)
![GitHub last commit](https://img.shields.io/github/last-commit/fckxorg/isp-clouds-test)

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

## Running in Docker container
```
docker build --tag ispras-test-server
docker run --publish 8080:8080 ispras-test-server
```
