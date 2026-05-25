package docs

import "embed"

// Assets contains the OpenAPI spec and Swagger UI static files.
//
//go:embed openapi.yaml swagger/*
var Assets embed.FS
