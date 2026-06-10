module koschei/web3hub

go 1.26

require (
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/lib/pq v1.10.9
	golang.org/x/crypto v0.23.0
)

replace github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api/pkg/agent => ./pkg/agent
replace github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api/pkg/audit => ./pkg/audit
replace github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api/pkg/utils => ./pkg/utils
