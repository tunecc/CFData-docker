package main

import (
	"math/rand/v2"
	"net"
	"strconv"
	"strings"
)

// 同网段最小间距乱序（学习 subs-check-pro proxy/shuffle.go SmartShuffleByServer 的思路，
// 见 docs/analyses/subs-check-pro-learnings.md 调查）：按 IPv4 /24（或主域）分组后
// 以"组内间隔铺设"重排，降低相邻并发测同网段节点被上游限速/测死的概率。
// 只改变被测顺序，不改变元素多重集合；结果展示排序由调用方自行保证。
// 分组铺设保证：只要该组能放下（(count-1)*(minSpacing+1)+1 ≤ n），组内元素两两
// 位置距离必 > minSpacing；放不下时对放不下的元素做最优努力回填，保证终止。

// subnetGroupKey 返回候选的分组键：IPv4 用 /24 网段（ip4: 前缀），
// 域名用主域（dom: 前缀），其余（IPv6、无点文本等）返回空串表示不参与间距约束。
// 输入允许带端口（host:port），会先剥离端口再识别。
func subnetGroupKey(candidate string) string {
	s := strings.TrimSpace(candidate)
	if s == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	if ip := net.ParseIP(s); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			prefix24 := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8
			return "ip4:" + strconv.FormatUint(uint64(prefix24), 10)
		}
		// IPv6 分布天然稀疏，不参与同网段间距约束
		return ""
	}
	s = strings.ToLower(s)
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return ""
	}
	return "dom:" + rootDomainOf(parts)
}

// rootDomainOf 启发式主域提取（对标 subs-check-pro extractRootDomain，不依赖
// public suffix 库）：两段直接返回；类似 .com.cn/.co.uk 的二级后缀取三段。
func rootDomainOf(parts []string) string {
	n := len(parts)
	if n <= 2 {
		return strings.Join(parts, ".")
	}
	switch parts[n-2] {
	case "com", "co", "org", "net", "edu", "gov", "ac":
		return strings.Join(parts[n-3:], ".")
	default:
		return strings.Join(parts[n-2:], ".")
	}
}

// shuffleMinSpacing 按候选规模与并发数推导最小间距（对标 subs-check-pro
// check/check.go:301 的 calcMinSpacing：max(并发*5, 总数/15)）。
func shuffleMinSpacing(threads, candidateCount int) int {
	if threads < 1 {
		threads = 1
	}
	spacing := threads * 5
	if bySize := candidateCount / 15; bySize > spacing {
		spacing = bySize
	}
	return spacing
}

// smartShuffleBySubnet 就地重排 items，使同一分组键的元素两两位置距离 > minSpacing
// （组可放下时严格成立，放不下时尽力）。keyFn 返回空串的元素不参与约束；
// minSpacing<=0 或元素数<2 时不处理；rnd 为 nil 时使用全局随机源。
func smartShuffleBySubnet[T any](items []T, minSpacing int, keyFn func(T) string, rnd *rand.Rand) {
	n := len(items)
	if n < 2 || minSpacing <= 0 {
		return
	}
	randomInt := func(upper int) int {
		if upper <= 0 {
			return 0
		}
		if rnd != nil {
			return rnd.IntN(upper)
		}
		return rand.IntN(upper)
	}
	shuffleElems := func(slice []T) {
		if rnd != nil {
			rnd.Shuffle(len(slice), func(i, j int) { slice[i], slice[j] = slice[j], slice[i] })
		} else {
			rand.Shuffle(len(slice), func(i, j int) { slice[i], slice[j] = slice[j], slice[i] })
		}
	}
	shuffleStrings := func(slice []string) {
		if rnd != nil {
			rnd.Shuffle(len(slice), func(i, j int) { slice[i], slice[j] = slice[j], slice[i] })
		} else {
			rand.Shuffle(len(slice), func(i, j int) { slice[i], slice[j] = slice[j], slice[i] })
		}
	}

	// 按分组键归堆；空键元素不参与间距约束
	grouped := map[string][]T{}
	var ungrouped []T
	for i := range items {
		if k := keyFn(items[i]); k != "" {
			grouped[k] = append(grouped[k], items[i])
		} else {
			ungrouped = append(ungrouped, items[i])
		}
	}

	// 逐组以 minSpacing+1 为步长铺设；组间起点错开由占用检查保证互不影响。
	// start 上限保证本组从 start 起能完整放下（可行时严格满足间距）。
	result := make([]T, n)
	occupied := make([]bool, n)
	// 铺设不完的元素（组放不下）与无分组元素一起随机回填剩余空位
	var leftover []T

	groupKeys := make([]string, 0, len(grouped))
	for k := range grouped {
		groupKeys = append(groupKeys, k)
	}
	shuffleStrings(groupKeys)

	for _, key := range groupKeys {
		elems := grouped[key]
		shuffleElems(elems)
		// 组内间距收敛：目标间距放不下该组时，收敛到本组在 n 个位置上的最大可行间距，
		// 保证"可满足时严格达标、不可满足时最大化分散"（否则整组会退化为随机回填、毫无分散）
		stride := minSpacing + 1
		if len(elems) > 1 {
			// (count-1)*(s+1)+1 ≤ n 时间距 s 可行 → s_max = (n-1)/(count-1) - 1
			maxSpacing := (n-1)/(len(elems)-1) - 1
			if maxSpacing < 0 {
				maxSpacing = 0
			}
			if maxSpacing < minSpacing {
				stride = maxSpacing + 1
				if stride < 1 {
					stride = 1
				}
			}
		}
		maxStart := n - 1 - (len(elems)-1)*stride
		start := 0
		if maxStart > 0 {
			start = randomInt(maxStart + 1)
		}
		last := -1
		for _, elem := range elems {
			pos := -1
			if last < 0 {
				for p := start; p < n; p++ {
					if !occupied[p] {
						pos = p
						break
					}
				}
			} else {
				ideal := last + stride
				if ideal > n-1 {
					ideal = n - 1
				}
				for p := ideal; p < n; p++ {
					if !occupied[p] && p-last >= stride {
						pos = p
						break
					}
				}
				if pos < 0 {
					for p := ideal - 1; p > last; p-- {
						if !occupied[p] && p-last >= stride {
							pos = p
							break
						}
					}
				}
			}
			if pos < 0 {
				leftover = append(leftover, elem)
				continue
			}
			result[pos] = elem
			occupied[pos] = true
			last = pos
		}
	}

	shuffleElems(ungrouped)
	leftover = append(leftover, ungrouped...)
	shuffleElems(leftover)
	next := 0
	for i := 0; i < n; i++ {
		if occupied[i] {
			continue
		}
		if next >= len(leftover) {
			break
		}
		result[i] = leftover[next]
		next++
	}
	copy(items, result)
}
