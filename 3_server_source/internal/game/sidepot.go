package game

import "sort"

// sidePot represents one pot layer and the players eligible to win it.
type sidePot struct {
	Pot     int64
	Players []*Player // eligible (non-folded players who contributed >= layer level)
}

// poolWinner is the result of awarding one side pot: which players split it.
type poolWinner struct {
	PoolID  int32
	Pot     int64
	Winners []*Player
}

// computeSidePots splits total contributions into main + side pots.
// Folded players still contribute to pots but cannot win them. The algorithm:
//  1. Collect all players who put chips in (TotalBet > 0), sorted by TotalBet asc.
//  2. For each distinct contribution level, the layer pot = (level - prevLevel) * numRemaining.
//  3. Eligible contestants for a layer = non-folded players with TotalBet >= level.
func computeSidePots(allPlayers []*Player) []sidePot {
	type contrib struct {
		p        *Player
		totalBet int64
	}
	var contribs []contrib
	for _, p := range allPlayers {
		if p.TotalBet > 0 {
			contribs = append(contribs, contrib{p, p.TotalBet})
		}
	}
	if len(contribs) == 0 {
		return nil
	}
	sort.Slice(contribs, func(i, j int) bool { return contribs[i].totalBet < contribs[j].totalBet })

	var pots []sidePot
	var prevLevel int64
	for i := 0; i < len(contribs); {
		level := contribs[i].totalBet
		if level <= prevLevel {
			i++
			continue
		}
		layer := (level - prevLevel) * int64(len(contribs)-i)
		// Eligible: non-folded players whose TotalBet >= level.
		var eligible []*Player
		for _, c := range contribs {
			if c.totalBet >= level && !c.p.Folded {
				eligible = append(eligible, c.p)
			}
		}
		pots = append(pots, sidePot{Pot: layer, Players: eligible})
		prevLevel = level
		// Skip all contribs at this level.
		for i < len(contribs) && contribs[i].totalBet == level {
			i++
		}
	}
	return pots
}

// awardSidePots evaluates each pot's eligible players and returns per-pool winners.
// Ties split the pot evenly (remainder lost to integer division, matching the
// official server's floor behaviour).
func awardSidePots(pots []sidePot, community []byte) []poolWinner {
	var results []poolWinner
	for i, pot := range pots {
		if len(pot.Players) == 0 {
			continue
		}
		if len(pot.Players) == 1 {
			results = append(results, poolWinner{PoolID: int32(i), Pot: pot.Pot, Winners: pot.Players})
			continue
		}
		var best EvalResult
		var winners []*Player
		for _, p := range pot.Players {
			cards := make([]byte, 0, len(p.Cards)+len(community))
			cards = append(cards, p.Cards...)
			cards = append(cards, community...)
			res := EvaluateHand(cards)
			if len(winners) == 0 || res.Rank > best.Rank || (res.Rank == best.Rank && res.Score > best.Score) {
				best = res
				winners = []*Player{p}
			} else if res.Rank == best.Rank && res.Score == best.Score {
				winners = append(winners, p)
			}
		}
		results = append(results, poolWinner{PoolID: int32(i), Pot: pot.Pot, Winners: winners})
	}
	return results
}
