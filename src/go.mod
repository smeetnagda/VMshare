module github.com/smeetnagda/vmshare

go 1.24.0

require (
	github.com/mattn/go-sqlite3 v1.14.28
	github.com/shirou/gopsutil v3.21.11+incompatible
)

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/sys v0.20.0 // indirect
)

replace github.com/smeetnagda/vmshare => ./

replace github.com/smeetnagda/vmshare/internal/agent => ./internal/agent

replace github.com/smeetnagda/vmshare/internal/multipass => ./internal/multipass

replace github.com/smeetnagda/vmshare/internal/server => ./internal/server
