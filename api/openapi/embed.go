package openapi

import "embed"

// FS contains the versioned OpenAPI documents embedded into the binary.
//go:embed v1/*
var FS embed.FS
