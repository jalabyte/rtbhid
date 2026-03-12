package httpapi

import _ "embed"

//go:embed web/index.html
var dashboardHTML []byte

//go:embed web/login.html
var loginHTML []byte
