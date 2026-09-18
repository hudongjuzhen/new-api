package runninghub

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Per-channel task concurrency gate.
//
// RunningHub rejects the request that exceeds an account's concurrency instead
// of queueing it upstream, so the gateway has to queue on its own. The limit is
// configured per channel (ChannelSettings.MaxConcurrency, 0 = unlimited) and a
// slot is occupied for the whole task lifetime — submit → terminal state — not
// for the HTTP request, because the upstream concurrency is consumed by running
// tasks.
//
// Occupancy is derived from the shared tasks table (every non-terminal task of
// the channel counts), so the accounting survives a restart and stays correct
// across instances. The only gap that the database cannot see is the window
// between "slot granted" and "task row persisted"; an in-process pending
// counter covers it.

const (
	// errorCodeChannelSaturated marks a saturated channel pool so the submit
	// handler can turn it into a queued task record (or, on the site-less path,
	// answer 429 instead of the generic channel error).
	errorCodeChannelSaturated = "channel_concurrency_saturated"
)

// errChannelSaturated is the sentinel wrapped into the selection error when no
// channel of the site has a free slot. The submit handler converts it into a
// QUEUED task record; see queue.go.
var errChannelSaturated = errors.New("channel concurrency saturated")

// channelSlot is a granted concurrency slot. release frees it exactly once, so
// callers may both release explicitly on failure and defer it as a safety net.
type channelSlot struct {
	channelID int
	released  bool
}

func (s *channelSlot) release() {
	if s == nil || s.released {
		return
	}
	s.released = true
	rhConcurrency.release(s.channelID)
}

// concurrencyGate holds the in-process reservations that the tasks table cannot
// see yet.
type concurrencyGate struct {
	mu      sync.Mutex
	pending map[int]int
}

var rhConcurrency = &concurrencyGate{pending: make(map[int]int)}

func (g *concurrencyGate) release(channelID int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	remaining := g.pending[channelID] - 1
	if remaining > 0 {
		g.pending[channelID] = remaining
		return
	}
	delete(g.pending, channelID)
}

// rhCandidate is one enabled channel of a site type plus its resolved cap.
type rhCandidate struct {
	channel  *model.Channel
	capacity int // 0 = unlimited
}

// inFlightTaskCounts returns, per channel, how many tasks are still occupying
// an upstream concurrency slot. One grouped query for the whole candidate set.
func inFlightTaskCounts(channelIDs []int) (map[int]int, error) {
	counts := make(map[int]int, len(channelIDs))
	if len(channelIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		ChannelId int
		Total     int64
	}
	err := db().Model(&model.Task{}).
		Select("channel_id, COUNT(*) AS total").
		Where("channel_id IN ?", channelIDs).
		Where("status NOT IN ?", []model.TaskStatus{model.TaskStatusSuccess, model.TaskStatusFailure}).
		Group("channel_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.ChannelId] = int(row.Total)
	}
	return counts, nil
}

// siteCandidates loads the enabled channels of a site's channel type together
// with their concurrency caps, ordered by priority then weight.
//
// Group filtering mirrors the host's token-group semantics. It deliberately
// keeps the historical fallback of the previous implementation: when no channel
// of the site matches the caller's group, the unfiltered pool is used instead
// of failing the request.
func siteCandidates(channelType int, group string) ([]rhCandidate, error) {
	var channels []model.Channel
	err := db().
		Where("type = ? AND status = ?", channelType, common.ChannelStatusEnabled).
		Order("priority desc, weight desc, id asc").
		Limit(100).
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	if group != "" {
		matched := make([]model.Channel, 0, len(channels))
		for _, ch := range channels {
			if common.StringsContains(ch.GetGroups(), group) {
				matched = append(matched, ch)
			}
		}
		if len(matched) > 0 {
			channels = matched
		}
	}
	cands := make([]rhCandidate, 0, len(channels))
	for i := range channels {
		ch := &channels[i]
		cands = append(cands, rhCandidate{channel: ch, capacity: ch.GetSetting().MaxConcurrency})
	}
	return cands, nil
}

// reserveSlot grants a slot on the best available channel, or returns nil when
// every candidate is saturated. Channels are walked in priority tiers starting
// at startTier; inside a tier the pick follows the host's weight semantics.
//
// The database read happens while the gate is locked: that is what keeps two
// concurrent submits of this process from claiming the same last slot. It
// serializes slot grants, which is acceptable for a queue whose whole point is
// a small upstream concurrency.
func reserveSlot(cands []rhCandidate, startTier int) (*model.Channel, *channelSlot, error) {
	if len(cands) == 0 {
		return nil, nil, nil
	}
	rhConcurrency.mu.Lock()
	defer rhConcurrency.mu.Unlock()

	channelIDs := make([]int, 0, len(cands))
	for _, cand := range cands {
		channelIDs = append(channelIDs, cand.channel.Id)
	}
	inFlight, err := inFlightTaskCounts(channelIDs)
	if err != nil {
		return nil, nil, err
	}

	for _, tier := range priorityTiers(cands, startTier) {
		free := make([]rhCandidate, 0, len(tier))
		for _, cand := range tier {
			if cand.capacity <= 0 {
				free = append(free, cand)
				continue
			}
			occupied := inFlight[cand.channel.Id] + rhConcurrency.pending[cand.channel.Id]
			if occupied < cand.capacity {
				free = append(free, cand)
			}
		}
		if len(free) == 0 {
			continue
		}
		picked := free[weightedPickIndex(free)]
		rhConcurrency.pending[picked.channel.Id]++
		return picked.channel, &channelSlot{channelID: picked.channel.Id}, nil
	}
	return nil, nil, nil
}

// priorityTiers groups candidates by priority, highest first, rotated so the
// tier a retry asks for is tried first (same tiering the host uses for retries).
func priorityTiers(cands []rhCandidate, startTier int) [][]rhCandidate {
	order := make([]int64, 0, len(cands))
	byPriority := make(map[int64][]rhCandidate, len(cands))
	for _, cand := range cands {
		priority := cand.channel.GetPriority()
		if _, seen := byPriority[priority]; !seen {
			order = append(order, priority)
		}
		byPriority[priority] = append(byPriority[priority], cand)
	}
	tiers := make([][]rhCandidate, 0, len(order))
	for _, priority := range order {
		tiers = append(tiers, byPriority[priority])
	}
	if len(tiers) == 0 {
		return nil
	}
	if startTier <= 0 {
		return tiers
	}
	if startTier >= len(tiers) {
		startTier = len(tiers) - 1
	}
	rotated := make([][]rhCandidate, 0, len(tiers))
	rotated = append(rotated, tiers[startTier:]...)
	rotated = append(rotated, tiers[:startTier]...)
	return rotated
}

// weightedPickIndex picks a candidate by channel weight, treating weight 0 as
// 100 so a weight-less pool stays uniform (matches the host's smoothing).
func weightedPickIndex(cands []rhCandidate) int {
	total := 0
	for _, cand := range cands {
		total += effectiveWeight(cand.channel)
	}
	if total <= 0 {
		return rand.Intn(len(cands))
	}
	point := rand.Intn(total)
	for i, cand := range cands {
		point -= effectiveWeight(cand.channel)
		if point < 0 {
			return i
		}
	}
	return len(cands) - 1
}

func effectiveWeight(ch *model.Channel) int {
	weight := ch.GetWeight()
	if weight <= 0 {
		return 100
	}
	return weight
}

// reserveChannelSlot reserves a slot on one specific channel, returning a nil
// slot when the channel is already at its cap. It serves the site-less (legacy)
// selection path, which has no candidate pool to fail over to.
func reserveChannelSlot(channel *model.Channel) (*channelSlot, error) {
	if channel == nil {
		return nil, nil
	}
	_, slot, err := reserveSlot([]rhCandidate{{
		channel:  channel,
		capacity: channel.GetSetting().MaxConcurrency,
	}}, 0)
	return slot, err
}

// saturatedError is the error returned when every channel of a site is at its
// cap. The site-scoped submit path converts it into a queued task; the site-less
// path reports it to the caller as 429.
func saturatedError(site string) error {
	target := siteName(site)
	if target == "" {
		target = "RunningHub"
	}
	return fmt.Errorf("%w: %s 上游并发已满，请稍后重试", errChannelSaturated, target)
}
