module koschei/api

go 1.26

require (
	github.com/lib/pq v1.10.9
	golang.org/x/sync v0.7.0
)


replace golang.org/x/sync => ./internal/xsync

