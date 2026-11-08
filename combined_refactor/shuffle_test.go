package main

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// shuffleIPKey 用候选文本（IP 或 IP:PORT）作为分组键来源。
func shuffleIPKey(s string) string { return subnetGroupKey(s) }

func sameSubnetPositions(items []string, target string) []int {
	key := subnetGroupKey(target)
	positions := make([]int, 0)
	for i, item := range items {
		if subnetGroupKey(item) == key {
			positions = append(positions, i)
		}
	}
	return positions
}

// groupSpacingViolations 返回同组元素中位置距离 <= minSpacing 的相邻对数量。
func groupSpacingViolations(positions []int, minSpacing int) int {
	violations := 0
	for i := 1; i < len(positions); i++ {
		if positions[i]-positions[i-1] <= minSpacing {
			violations++
		}
	}
	return violations
}

// A1：同组元素两两位置距离 > minSpacing（等价地任意 minSpacing+1 窗口内同组 ≤2），
// 总数与多重集合不变，顺序被打乱。参数可满足时（10 条同 /24，间距 10，需距离 ≥11，
// 100 个位置恰好容纳）必须达成。其余候选用无点文本（不参与任何分组），保证目标组
// 是唯一分组、铺设不被其他组占用干扰。
func TestSmartShuffleBySubnetSpacingSatisfied(t *testing.T) {
	items := make([]string, 0, 100)
	for i := range 90 {
		items = append(items, fmt.Sprintf("host-%02d", i)) // 无点：不参与分组约束
	}
	for i := range 10 {
		items = append(items, ipPortString(1, 2, 3, i+1, 443)) // 同属 1.2.3.0/24
	}

	original := append([]string(nil), items...)
	for seed := uint64(1); seed <= 20; seed++ {
		work := append([]string(nil), items...)
		smartShuffleBySubnet(work, 10, shuffleIPKey, rand.New(rand.NewPCG(seed, seed*7)))

		if len(work) != len(original) {
			t.Fatalf("seed %d: 长度改变 %d -> %d", seed, len(original), len(work))
		}
		counts := map[string]int{}
		for _, v := range original {
			counts[v]++
		}
		for _, v := range work {
			counts[v]--
			if counts[v] < 0 {
				t.Fatalf("seed %d: 出现原集合中不存在的元素 %s", seed, v)
			}
		}
		for v, c := range counts {
			if c != 0 {
				t.Fatalf("seed %d: 元素 %s 数量不守恒（余 %d）", seed, v, c)
			}
		}

		positions := sameSubnetPositions(work, "1.2.3.1")
		if len(positions) != 10 {
			t.Fatalf("seed %d: 同组元素数量错误 %d", seed, len(positions))
		}
		if violations := groupSpacingViolations(positions, 10); violations != 0 {
			t.Fatalf("seed %d: 存在 %d 处同组元素间距 ≤ 10（位置 %v）", seed, violations, positions)
		}
		// 顺序确实被打乱（固定种子下确定性结果不应等于原顺序）
		identical := true
		for i := range work {
			if work[i] != original[i] {
				identical = false
				break
			}
		}
		if identical {
			t.Fatalf("seed %d: 乱序后顺序与原始输入完全相同", seed)
		}
	}
}

// 小数组（<2）不处理；minSpacing<=0 不处理。
func TestSmartShuffleBySubnetDegenerateSmallInput(t *testing.T) {
	single := []string{"1.2.3.1:443"}
	smartShuffleBySubnet(single, 10, shuffleIPKey, rand.New(rand.NewPCG(1, 2)))
	if single[0] != "1.2.3.1:443" {
		t.Fatalf("单元素不应被改动: %s", single[0])
	}

	pair := []string{"1.2.3.1:443", "9.9.9.9:443"}
	smartShuffleBySubnet(pair, 0, shuffleIPKey, rand.New(rand.NewPCG(1, 2)))
	if pair[0] != "1.2.3.1:443" || pair[1] != "9.9.9.9:443" {
		t.Fatalf("minSpacing=0 不应处理: %v", pair)
	}
}

// 全部同 /24：间距不可满足，算法必须终止且多重集合不变。
func TestSmartShuffleBySubnetDegenerateAllSameSubnet(t *testing.T) {
	items := make([]string, 0, 50)
	for i := range 50 {
		items = append(items, ipPortString(1, 2, 3, i+1, 443))
	}
	original := append([]string(nil), items...)
	smartShuffleBySubnet(items, 10, shuffleIPKey, rand.New(rand.NewPCG(3, 4)))

	if len(items) != len(original) {
		t.Fatalf("长度改变: %d -> %d", len(original), len(items))
	}
	counts := map[string]int{}
	for _, v := range original {
		counts[v]++
	}
	for _, v := range items {
		counts[v]--
		if counts[v] != 0 {
			t.Fatalf("多重集合不守恒: %s", v)
		}
	}
}

// 不可满足参数（100 条中 30 条同 /24、间距 20）必须终止且多重集合不变。
func TestSmartShuffleBySubnetUnsatisfiableTerminates(t *testing.T) {
	items := make([]string, 0, 100)
	for i := range 70 {
		items = append(items, ipPortString(10, 0, i/250+1, i%250+2, 443+i))
	}
	for i := range 30 {
		items = append(items, ipPortString(1, 2, 3, i+1, 443))
	}
	original := append([]string(nil), items...)
	smartShuffleBySubnet(items, 20, shuffleIPKey, rand.New(rand.NewPCG(5, 6)))

	if len(items) != len(original) {
		t.Fatalf("长度改变: %d -> %d", len(original), len(items))
	}
	counts := map[string]int{}
	for _, v := range original {
		counts[v]++
	}
	for _, v := range items {
		counts[v]--
		if counts[v] != 0 {
			t.Fatalf("多重集合不守恒: %s", v)
		}
	}
}

// 多组共存：每个小组元素数远小于 n 时，各组间距都应满足。
func TestSmartShuffleBySubnetMultipleGroups(t *testing.T) {
	items := make([]string, 0, 100)
	for i := range 91 {
		items = append(items, fmt.Sprintf("host-%02d", i))
	}
	for i := range 3 {
		items = append(items, ipPortString(10, 0, 0, i+1, 443)) // 10.0.0.0/24
	}
	for i := range 3 {
		items = append(items, ipPortString(10, 0, 1, i+1, 443)) // 10.0.1.0/24
	}
	for i := range 3 {
		items = append(items, ipPortString(10, 0, 2, i+1, 443)) // 10.0.2.0/24
	}

	for seed := uint64(1); seed <= 20; seed++ {
		work := append([]string(nil), items...)
		smartShuffleBySubnet(work, 5, shuffleIPKey, rand.New(rand.NewPCG(seed, seed*13)))
		for _, probe := range []string{"10.0.0.1", "10.0.1.1", "10.0.2.1"} {
			positions := sameSubnetPositions(work, probe)
			if len(positions) != 3 {
				t.Fatalf("seed %d: 组 %s 元素数量错误 %d", seed, probe, len(positions))
			}
			if violations := groupSpacingViolations(positions, 5); violations != 0 {
				t.Fatalf("seed %d: 组 %s 存在 %d 处间距 ≤ 5（位置 %v）", seed, probe, violations, positions)
			}
		}
	}
}

// subnetGroupKey 分组行为：IPv4 /24 归组、带端口剥离、域名主域、IPv6/无点文本不参与。
func TestSubnetGroupKey(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.2.3.4", "ip4:16909056"},       // 1<<24|2<<16|3<<8
		{"1.2.3.9:443", "ip4:16909056"},   // 同 /24
		{"1.2.4.9:443", "ip4:16909312"},   // 不同 /24
		{"us1.example.com", "dom:example.com"},
		{"US1.Example.COM:8443", "dom:example.com"},
		{"www.google.com.cn", "dom:google.com.cn"},
		{"2001:db8::1", ""},
		{"localhost", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := subnetGroupKey(c.in); got != c.want {
			t.Errorf("subnetGroupKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ipPortString 生成测试用 ip:port 文本。
func ipPortString(a, b, c, d, port int) string {
	return fmt.Sprintf("%d.%d.%d.%d:%d", a, b, c, d, port)
}

// 不可满足参数下应"最大化分散"而非整组随机回填：间距收敛到本组最大可行间距后，
// 同组元素两两距离应达到收敛间距（此处 30 条同 /24、n=100 → 收敛间距 2，距离 ≥3）。
func TestSmartShuffleBySubnetUnsatisfiableStillSpreads(t *testing.T) {
	items := make([]string, 0, 100)
	for i := range 70 {
		items = append(items, fmt.Sprintf("host-%02d", i)) // 无点：不参与分组
	}
	for i := range 30 {
		items = append(items, ipPortString(1, 2, 3, i+1, 443)) // 同属 1.2.3.0/24
	}
	for seed := uint64(1); seed <= 20; seed++ {
		work := append([]string(nil), items...)
		smartShuffleBySubnet(work, 20, shuffleIPKey, rand.New(rand.NewPCG(seed, seed*11)))
		positions := sameSubnetPositions(work, "1.2.3.1")
		if len(positions) != 30 {
			t.Fatalf("seed %d: 同组元素数量错误 %d", seed, len(positions))
		}
		if violations := groupSpacingViolations(positions, 2); violations != 0 {
			t.Fatalf("seed %d: 不可满足参数下未最大化分散，存在 %d 处距离 ≤ 2（位置 %v）", seed, violations, positions)
		}
	}
}
