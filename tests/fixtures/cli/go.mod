// go.mod of the go.sum fixtures used by engine/cli (check-plugin --fix): direct requires are fixed with
// "go get", indirect ones with "go mod edit --replace".
module example.com/plugin

go 1.22

require golang.org/x/mod v0.6.10-dev.0.20220419223038-86c51ed26bb4

require (
	cloud.google.com/go v0.100.3 // indirect
	github.com/Azure/azure-sdk-for-go v59.3.1+incompatible // indirect
)
