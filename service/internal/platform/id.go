package platform

import (
	"errors"
	"fmt"
	"regexp"
)

// 命名空间 ID 的格式，与 docs/PROTOCOL.md §2 保持一致。
var (
	idPattern       = regexp.MustCompile(`^([a-z][a-z0-9_]{1,15}):([A-Za-z0-9_-]{1,64})$`)
	platformPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,15}$`)
)

// MakeID 拼出带命名空间的 ID。
// 它不做校验：调用方应已经用 SplitID 或 IsValidID 检查过。
func MakeID(platform, external string) string {
	return platform + ":" + external
}

// SplitID 拆出平台名与平台内标识。
func SplitID(id string) (platformName, external string, err error) {
	m := idPattern.FindStringSubmatch(id)
	if m == nil {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	return m[1], m[2], nil
}

// IsValidID 判断一个 ID 是否符合命名空间规则。
func IsValidID(id string) bool {
	return idPattern.MatchString(id)
}

// ErrInvalidID 供上层用 errors.Is 判断，不直接返回给客户端。
var ErrInvalidID = errors.New("platform: 非法 ID")
