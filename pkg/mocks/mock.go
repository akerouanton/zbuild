package mocks

//go:generate mockgen -destination=./mock_filefetch.go -package mocks github.com/NiR-/webdf/pkg/filefetch FileFetcher
//go:generate mockgen -destination=./mock_pkgsolver.go -package mocks github.com/NiR-/webdf/pkg/pkgsolver PackageSolver
