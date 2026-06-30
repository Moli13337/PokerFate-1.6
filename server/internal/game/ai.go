package game

import "math/rand"

// aiDecide picks an action for an AI player using Monte Carlo win-rate
// estimation, pot odds and a small bluff factor. Returns (actionType, chips)
// where chips is the raise increment (not the total bet) for actionRaise.
func (m *Manager) aiDecide(room *Room, player *Player, callNeed, minChipin int64) (actionType int32, chips int64) {
	table := room.Table

	// Can't act without hole cards.
	if len(player.Cards) < 2 {
		return actionFold, 0
	}

	// Estimate win probability via Monte Carlo.
	winRate := monteCarloWinRate(player.Cards, table.Community, 100)

	// Position adjustment: dealer plays looser, blinds tighter.
	switch player.SeatID {
	case table.Dealer:
		winRate *= 0.9
	case table.SBSeat, table.BBSeat:
		winRate *= 1.1
	}

	// Pot odds: required equity to break even on a call.
	potOdds := 0.0
	if table.Pot+callNeed > 0 {
		potOdds = float64(callNeed) / float64(table.Pot+callNeed)
	}

	// Bluff: 10% chance to raise with a weak hand.
	if minChipin > 0 && player.Gold > callNeed+minChipin && rand.Intn(10) == 0 {
		return actionRaise, minChipin
	}

	// Decision thresholds.
	switch {
	case winRate > 0.75:
		// Strong: raise or all-in.
		if minChipin > 0 && player.Gold > callNeed+minChipin {
			raise := minChipin * int64(1+rand.Intn(3))
			if raise > player.Gold-callNeed {
				raise = player.Gold - callNeed
			}
			if raise >= minChipin {
				return actionRaise, raise
			}
		}
		if player.Gold > 0 {
			return actionAllIn, player.Gold
		}
		return actionFold, 0

	case winRate > 0.55:
		// Medium: call, or small raise if affordable.
		if minChipin > 0 && player.Gold > callNeed+minChipin && rand.Intn(3) == 0 {
			return actionRaise, minChipin
		}
		if callNeed == 0 {
			return actionCall, 0
		}
		if player.Gold >= callNeed {
			return actionCall, 0
		}
		if player.Gold > 0 {
			return actionAllIn, player.Gold
		}
		return actionFold, 0

	case winRate > potOdds:
		// Marginal + profitable call.
		if callNeed == 0 {
			return actionCall, 0
		}
		if player.Gold >= callNeed {
			return actionCall, 0
		}
		if player.Gold > 0 {
			return actionAllIn, player.Gold
		}
		return actionFold, 0

	default:
		// Weak: check if free, else fold.
		if callNeed == 0 {
			return actionCall, 0
		}
		return actionFold, 0
	}
}

// monteCarloWinRate estimates the probability that hole+community beats a
// random opponent hand by sampling `iterations` deals from the remaining deck.
func monteCarloWinRate(hole []byte, community []byte, iterations int) float64 {
	if len(hole) < 2 {
		return 0
	}

	// Build the remaining deck: all 52 cards minus hole and community.
	used := make(map[byte]bool, len(hole)+len(community))
	for _, c := range hole {
		used[c] = true
	}
	for _, c := range community {
		used[c] = true
	}
	var deck []byte
	for i := 0; i < 52; i++ {
		if !used[byte(i)] {
			deck = append(deck, byte(i))
		}
	}

	holeCopy := append(append([]byte{}, hole...), community...)
	oppHole := make([]byte, 2)
	var wins, ties int
	for i := 0; i < iterations; i++ {
		// Shuffle a copy of the remaining deck.
		shuffled := append([]byte{}, deck...)
		rand.Shuffle(len(shuffled), func(a, b int) {
			shuffled[a], shuffled[b] = shuffled[b], shuffled[a]
		})

		oppHole[0] = shuffled[0]
		oppHole[1] = shuffled[1]
		need := 5 - len(community)
		fullBoard := append(append([]byte{}, community...), shuffled[2:2+need]...)

		myEval := EvaluateHand(append(append([]byte{}, holeCopy...), fullBoard...))
		oppEval := EvaluateHand(append(append([]byte{}, oppHole...), fullBoard...))

		if myEval.Rank > oppEval.Rank || (myEval.Rank == oppEval.Rank && myEval.Score > oppEval.Score) {
			wins++
		} else if myEval.Rank == oppEval.Rank && myEval.Score == oppEval.Score {
			ties++
		}
	}
	return (float64(wins) + 0.5*float64(ties)) / float64(iterations)
}
