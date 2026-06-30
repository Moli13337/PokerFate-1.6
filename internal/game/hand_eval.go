package game

import "sort"

type HandRank int

const (
	HighCard HandRank = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
	RoyalFlush
)

type EvalResult struct {
	Rank  HandRank
	Score uint32
}

func cardRank(c byte) int { return int(c % 13) }
func cardSuit(c byte) int { return int(c / 13) }

func EvaluateHand(cards []byte) EvalResult {
	if len(cards) < 5 {
		return EvalResult{Rank: HighCard, Score: 0}
	}

	if len(cards) == 5 {
		return evaluate5(cards)
	}

	best := EvalResult{Rank: HighCard, Score: 0}
	n := len(cards)
	combos := combinations(n, 5)
	for _, combo := range combos {
		hand := make([]byte, 5)
		for i, idx := range combo {
			hand[i] = cards[idx]
		}
		r := evaluate5(hand)
		if r.Rank > best.Rank || (r.Rank == best.Rank && r.Score > best.Score) {
			best = r
		}
	}
	return best
}

func evaluate5(cards []byte) EvalResult {
	ranks := make([]int, 5)
	suits := make([]int, 5)
	for i, c := range cards {
		ranks[i] = cardRank(c)
		suits[i] = cardSuit(c)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(ranks)))

	isFlush := suits[0] == suits[1] && suits[1] == suits[2] && suits[2] == suits[3] && suits[3] == suits[4]

	isStraight := false
	straightHigh := 0
	if ranks[0]-ranks[4] == 4 && ranks[0] != ranks[1] && ranks[1] != ranks[2] && ranks[2] != ranks[3] && ranks[3] != ranks[4] {
		isStraight = true
		straightHigh = ranks[0]
	}
	if ranks[0] == 12 && ranks[1] == 3 && ranks[2] == 2 && ranks[3] == 1 && ranks[4] == 0 {
		isStraight = true
		straightHigh = 3
	}

	counts := make(map[int]int)
	for _, r := range ranks {
		counts[r]++
	}

	var groups [][2]int
	for r, c := range counts {
		groups = append(groups, [2]int{r, c})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i][1] != groups[j][1] {
			return groups[i][1] > groups[j][1]
		}
		return groups[i][0] > groups[j][0]
	})

	score := func(vals ...int) uint32 {
		var s uint32
		for _, v := range vals {
			s = s*13 + uint32(v)
		}
		return s
	}

	if isFlush && isStraight {
		if straightHigh == 12 {
			return EvalResult{Rank: RoyalFlush, Score: score(straightHigh)}
		}
		return EvalResult{Rank: StraightFlush, Score: score(straightHigh)}
	}

	if len(groups) >= 1 && groups[0][1] == 4 {
		return EvalResult{Rank: FourOfAKind, Score: score(groups[0][0], groups[1][0])}
	}

	if len(groups) >= 2 && groups[0][1] == 3 && groups[1][1] == 2 {
		return EvalResult{Rank: FullHouse, Score: score(groups[0][0], groups[1][0])}
	}

	if isFlush {
		return EvalResult{Rank: Flush, Score: score(ranks...)}
	}

	if isStraight {
		return EvalResult{Rank: Straight, Score: score(straightHigh)}
	}

	if len(groups) >= 1 && groups[0][1] == 3 {
		kickers := []int{groups[0][0]}
		for _, g := range groups[1:] {
			kickers = append(kickers, g[0])
		}
		return EvalResult{Rank: ThreeOfAKind, Score: score(kickers...)}
	}

	if len(groups) >= 2 && groups[0][1] == 2 && groups[1][1] == 2 {
		high := groups[0][0]
		low := groups[1][0]
		if low > high {
			high, low = low, high
		}
		return EvalResult{Rank: TwoPair, Score: score(high, low, groups[2][0])}
	}

	if len(groups) >= 1 && groups[0][1] == 2 {
		kickers := []int{groups[0][0]}
		for _, g := range groups[1:] {
			kickers = append(kickers, g[0])
		}
		return EvalResult{Rank: OnePair, Score: score(kickers...)}
	}

	return EvalResult{Rank: HighCard, Score: score(ranks...)}
}

func combinations(n, k int) [][]int {
	var result [][]int
	var combo []int
	var backtrack func(start int)
	backtrack = func(start int) {
		if len(combo) == k {
			c := make([]int, k)
			copy(c, combo)
			result = append(result, c)
			return
		}
		for i := start; i < n; i++ {
			combo = append(combo, i)
			backtrack(i + 1)
			combo = combo[:len(combo)-1]
		}
	}
	backtrack(0)
	return result
}
