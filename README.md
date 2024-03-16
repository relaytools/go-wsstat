# go-wsstat [![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)][license]

[license]: /LICENSE

Use the `go-wsstat` Golang package to trace WebSocket connection and latency in your Go applications. It wraps the [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket) package for the actual WebSocket connection implementation, and measures the duration of the different phases of the connection cycle. The program takes inspiration from the [go-httpstat](https://github.com/tcnksm/go-httpstat) package, which I've found useful for tracing HTTP connections.

## Install

The package is installed using the `go get` command:

```bash
go get github.com/jakobilobi/go-wsstat
```

## Usage

The [_example/main.go](./_example/main.go) program demonstrates how to use the `go-wsstat` package to trace a WebSocket connection.

Run the example:

```bash
go run _example/main.go <a WebSocket URL>
```
