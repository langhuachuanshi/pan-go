package lanzou

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// reArg1 提取 acw_sc__v2 挑战页 arg1 参数的正则
var reArg1 = regexp.MustCompile(`var\s+arg1\s*=\s*'([A-F0-9]{40})'`)

// solveAcwScV2 计算 acw_sc__v2 cookie 值
func solveAcwScV2(html string, cfg *ChallengeConfig) (string, error) {
	if cfg == nil {
		cfg = DefaultChallengeConfig()
	}

	matches := reArg1.FindStringSubmatch(html)
	if len(matches) < 2 {
		return "", fmt.Errorf("arg1 not found in challenge page")
	}
	arg1 := matches[1]

	var q [40]byte
	for x := 0; x < len(arg1); x++ {
		for z := 0; z < len(cfg.Perm); z++ {
			if cfg.Perm[z] == x+1 {
				q[z] = arg1[x]
			}
		}
	}
	u := string(q[:])

	xorKey := cfg.XORKey
	var v strings.Builder
	for x := 0; x < len(u) && x < len(xorKey); x += 2 {
		a, _ := strconv.ParseUint(u[x:x+2], 16, 8)
		b, _ := strconv.ParseUint(xorKey[x:x+2], 16, 8)
		v.WriteString(fmt.Sprintf("%02x", a^b))
	}
	return v.String(), nil
}

// isChallengePage 检查是否为JS挑战页面
func isChallengePage(html string) bool {
	return strings.Contains(html, "var arg1=") && strings.Contains(html, "acw_sc__v2")
}
