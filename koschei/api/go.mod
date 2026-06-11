module koschei/api

go 1.23

require (
	github.com/lib/pq v1.10.9
	github.com/mr-tron/base58 v1.3.0
	golang.org/x/sync v0.7.0
)

replace golang.org/x/sync => ./internal/xsync

replace github.com/mr-tron/base58 => ./internal/third_party/base58
