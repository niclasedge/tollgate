// Package schema embeds the JSON Schema of the versioned runtime
// configuration (ADR-0008). The same bytes validate YAML imports in the
// backend and are served to the frontend editor.
package schema

import _ "embed"

// URL is the $id of the schema.
const URL = "https://github.com/niclasedge/tollgate/config/schema/tollgate-config.v1.schema.json"

// ConfigV1 is tollgate-config.v1.schema.json (JSON Schema 2020-12).
//
//go:embed tollgate-config.v1.schema.json
var ConfigV1 []byte
