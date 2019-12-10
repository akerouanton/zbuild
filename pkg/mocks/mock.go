package mocks

//go:generate mockgen -destination=./mock_statesolver.go -package mocks github.com/NiR-/zbuild/pkg/statesolver StateSolver
//go:generate mockgen -destination=./mock_pkgsolver.go -package mocks github.com/NiR-/zbuild/pkg/pkgsolver PackageSolver
//go:generate mockgen -destination=./mock_registry.go -package mocks github.com/NiR-/zbuild/pkg/registry KindHandler
