module github.com/skudra-go/ant-examples/ledger

go 1.27

require (
	github.com/jackc/pgx/v5 v5.11.0
	github.com/skudra-go/ant v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

// ant is unpublished for now, so this builds against a checkout beside this
// repository. Once ant is tagged, delete this line and require the tag.
replace github.com/skudra-go/ant => ../../../ant
