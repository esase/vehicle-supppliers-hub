# Generate types
oapi-codegen \
	-generate "types" \
	-o "internal/schema/gen_schema.go" \
	-package "schema" \
	"api/openapi.json"

# Generate spec
oapi-codegen \
	-generate "spec" \
	-o "internal/web/spec/oa_spec_gen.go" \
	-package "spec" \
	"api/openapi.json"


# user-service
oapi-codegen \
	-generate "types" \
	-o "internal/tools/client/userservice/openapi/types_gen.go" \
	-package "openapi" \
	"api/external/userservice.json"

oapi-codegen \
	-generate "client" \
	-o "internal/tools/client/userservice/openapi/client_gen.go" \
	-package "openapi" \
	"api/external/userservice.json"


# spit
oapi-codegen \
	-generate "types" \
	-o "internal/tools/client/spit/openapi/types_gen.go" \
	-package "openapi" \
	"api/external/spit.json"

oapi-codegen \
	-generate "client" \
	-o "internal/tools/client/spit/openapi/client_gen.go" \
	-package "openapi" \
	"api/external/spit.json"
