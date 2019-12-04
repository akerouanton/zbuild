package mocks

//go:generate mockgen -destination=./mock_filefetch.go -package mocks github.com/NiR-/zbuild/pkg/filefetch FileFetcher
//go:generate mockgen -destination=./mock_pkgsolver.go -package mocks github.com/NiR-/zbuild/pkg/pkgsolver PackageSolver
//go:generate mockgen -destination=./mock_registry.go -package mocks github.com/NiR-/zbuild/pkg/registry KindHandler
