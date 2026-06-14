package user

import "embed"

// PageFS 内嵌 user 后台页面配置。
//
//go:embed front/page/*/*/*.json
var PageFS embed.FS
