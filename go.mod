module github.com/protobuf-orm/protobuf-orm

go 1.26.2

require google.golang.org/protobuf v1.36.11

require github.com/lesomnus/protobuf-patch v0.0.0-20260803175157-e1b7a0c2804f // indirect

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.10.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

tool google.golang.org/protobuf/cmd/protoc-gen-go
