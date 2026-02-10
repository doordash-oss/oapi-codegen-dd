package testcase

//go:generate cp ../../testcase/gen.go ./gen.go
//go:generate cp ../../testcase/service.go.src ./service.go
//go:generate go run github.com/doordash-oss/oapi-codegen-dd/v3/cmd/oapi-codegen -config cfg.yml ../../api.yml
