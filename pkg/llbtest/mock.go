package llbtest

//go:generate mockgen -destination=./mock_client.go -package llbtest github.com/moby/buildkit/frontend/gateway/client Client,Reference
